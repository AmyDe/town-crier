package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/httputil"
	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// maxOfferCodeCount bounds a single generate request to 1000 codes.
const maxOfferCodeCount = 1000

// defaultMaxRedemptions is the redemption cap applied when a mint request
// omits maxRedemptions — the single-use case every code used to default to.
const defaultMaxRedemptions = 1

// minRequestMaxRedemptions and maxRequestMaxRedemptions bound the
// maxRedemptions field on a mint request, mirroring offercodes.NewOfferCode's
// own bounds so a violation is reported here as a clean 400 rather than
// surfacing as a 500 from the domain constructor.
const (
	minRequestMaxRedemptions = 1
	maxRequestMaxRedemptions = 10000
)

// defaultListLimit bounds GET /v1/admin/offer-codes when the caller omits
// ?limit. The table is admin-minted and small, so there is no pagination
// beyond this cap.
const defaultListLimit = 500

type generateRequest struct {
	Count          int    `json:"count"`
	Tier           string `json:"tier"`
	DurationDays   int    `json:"durationDays"`
	Label          string `json:"label"`
	MaxRedemptions *int   `json:"maxRedemptions"`
}

// apiErrorResponse is the { error, message:null } envelope the generate endpoint
// returns for validation failures.
type apiErrorResponse struct {
	Error   string  `json:"error"`
	Message *string `json:"message"`
}

// generateOfferCodes implements POST /v1/admin/offer-codes: mint N codes for a
// paid tier, all sharing one label and redemption cap, and return them, one
// display-formatted code per line, as text/plain. Validation failure messages
// include the " (Parameter 'command')" and "Actual value was N." suffixes for
// client compatibility.
func (h *handler) generateOfferCodes(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req generateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	tier, err := profiles.ParseSubscriptionTier(req.Tier)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Validation order: count, tier, duration, label, maxRedemptions.
	if req.Count < 1 || req.Count > maxOfferCodeCount {
		h.writeBadRequest(r, w, argOutOfRange("Count must be between 1 and 1000.", req.Count))
		return
	}
	if tier == profiles.TierFree {
		h.writeBadRequest(r, w, argException("Offer codes cannot grant the Free tier."))
		return
	}
	if req.DurationDays < 1 || req.DurationDays > 365 {
		h.writeBadRequest(r, w, argOutOfRange("DurationDays must be between 1 and 365.", req.DurationDays))
		return
	}
	if strings.TrimSpace(req.Label) == "" {
		h.writeBadRequest(r, w, argException("Label is required."))
		return
	}
	maxRedemptions := defaultMaxRedemptions
	if req.MaxRedemptions != nil {
		maxRedemptions = *req.MaxRedemptions
	}
	if maxRedemptions < minRequestMaxRedemptions || maxRedemptions > maxRequestMaxRedemptions {
		h.writeBadRequest(r, w, argOutOfRange("MaxRedemptions must be between 1 and 10000.", maxRedemptions))
		return
	}

	now := h.now()
	formatted := make([]string, 0, req.Count)
	for range req.Count {
		canonical, err := h.generator.Generate()
		if err != nil {
			h.serverError(w, r, "generate offer code", err)
			return
		}
		code, err := offercodes.NewOfferCode(canonical, tier, req.DurationDays, req.Label, maxRedemptions, now)
		if err != nil {
			h.serverError(w, r, "build offer code", err)
			return
		}
		if err := h.codes.Save(r.Context(), code); err != nil {
			h.serverError(w, r, "save offer code", err)
			return
		}
		formatted = append(formatted, offercodes.Format(canonical))
	}

	body := strings.Join(formatted, "\n") + "\n"
	// Bare "text/plain" (no charset): the contract test (tc-52t6) requires no
	// charset suffix; Go's default would append "; charset=utf-8".
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(body)); err != nil {
		h.logger.ErrorContext(r.Context(), "write offer codes", "error", err)
	}
}

// listedOfferCode is one row of the GET /v1/admin/offer-codes response.
type listedOfferCode struct {
	Code            string  `json:"code"`
	Label           string  `json:"label"`
	Tier            string  `json:"tier"`
	DurationDays    int     `json:"durationDays"`
	MaxRedemptions  int     `json:"maxRedemptions"`
	RedemptionCount int     `json:"redemptionCount"`
	CreatedAt       string  `json:"createdAt"` // RFC3339
	LastRedeemedAt  *string `json:"lastRedeemedAt"`
}

// listOfferCodes implements GET /v1/admin/offer-codes?label=<substring>&limit=<n>:
// codes ordered created_at DESC, optionally filtered to labels containing a
// case-insensitive substring. limit defaults to 500; an unparseable limit is a
// bodyless 400.
func (h *handler) listOfferCodes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var labelFilter *string
	if raw := q.Get("label"); raw != "" {
		labelFilter = &raw
	}

	limit := defaultListLimit
	if raw := q.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		limit = n
	}

	codes, err := h.codes.List(r.Context(), labelFilter, limit)
	if err != nil {
		h.serverError(w, r, "list offer codes", err)
		return
	}

	items := make([]listedOfferCode, 0, len(codes))
	for _, c := range codes {
		items = append(items, listedOfferCode{
			Code:            offercodes.Format(c.Code),
			Label:           c.Label,
			Tier:            c.Tier.String(),
			DurationDays:    c.DurationDays,
			MaxRedemptions:  c.MaxRedemptions,
			RedemptionCount: c.RedemptionCount,
			CreatedAt:       c.CreatedAt.Format(time.RFC3339),
			LastRedeemedAt:  formatOptionalTime(c.LastRedeemedAt),
		})
	}
	h.writeJSON(r, w, items)
}

// formatOptionalTime renders t as RFC3339, or nil when t itself is nil (a code
// that has never been redeemed).
func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

func (h *handler) writeBadRequest(r *http.Request, w http.ResponseWriter, message string) {
	body, err := httputil.EncodeJSON(apiErrorResponse{Error: message, Message: nil})
	if err != nil {
		h.serverError(w, r, "encode error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write admin error body", "error", err)
	}
}

// argOutOfRange formats an out-of-range validation message in the shape
// "{message} (Parameter 'command')\nActual value was {value}.".
func argOutOfRange(message string, value int) string {
	return message + " (Parameter 'command')\nActual value was " + strconv.Itoa(value) + "."
}

// argException formats a validation message in the shape
// "{message} (Parameter 'command')".
func argException(message string) string {
	return message + " (Parameter 'command')"
}
