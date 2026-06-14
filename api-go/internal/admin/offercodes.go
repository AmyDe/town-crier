package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// maxOfferCodeCount bounds a single generate request, mirroring the .NET
// GenerateOfferCodesCommandHandler.MaxCount.
const maxOfferCodeCount = 1000

type generateRequest struct {
	Count        int    `json:"count"`
	Tier         string `json:"tier"`
	DurationDays int    `json:"durationDays"`
}

// apiErrorResponse mirrors the .NET ApiErrorResponse { error, message:null } the
// generate endpoint returns for validation failures.
type apiErrorResponse struct {
	Error   string  `json:"error"`
	Message *string `json:"message"`
}

// generateOfferCodes implements POST /v1/admin/offer-codes: mint N codes for a
// paid tier and return them, one display-formatted code per line, as text/plain.
// Validation failures mirror the .NET handler's ArgumentException /
// ArgumentOutOfRangeException messages byte-for-byte (including the runtime's
// " (Parameter 'command')" and "Actual value was N." suffixes).
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

	// Validation order matches the .NET handler: count, then tier, then duration.
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

	now := h.now()
	formatted := make([]string, 0, req.Count)
	for range req.Count {
		canonical, err := h.generator.Generate()
		if err != nil {
			h.serverError(w, r, "generate offer code", err)
			return
		}
		code, err := offercodes.NewOfferCode(canonical, tier, req.DurationDays, now)
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
	// Bare "text/plain" (no charset) to match .NET Results.Text(body,
	// contentType: "text/plain") byte-for-byte — verified by the contract diff
	// (tc-52t6); Go's default would append "; charset=utf-8".
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(body)); err != nil {
		h.logger.ErrorContext(r.Context(), "write offer codes", "error", err)
	}
}

func (h *handler) writeBadRequest(r *http.Request, w http.ResponseWriter, message string) {
	body, err := encodeJSON(apiErrorResponse{Error: message, Message: nil})
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

// argOutOfRange reproduces .NET ArgumentOutOfRangeException.Message for
// nameof(command): "{message} (Parameter 'command')\nActual value was {value}.".
func argOutOfRange(message string, value int) string {
	return message + " (Parameter 'command')\nActual value was " + strconv.Itoa(value) + "."
}

// argException reproduces .NET ArgumentException.Message for nameof(command):
// "{message} (Parameter 'command')".
func argException(message string) string {
	return message + " (Parameter 'command')"
}
