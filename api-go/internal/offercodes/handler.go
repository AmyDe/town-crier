package offercodes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

const maxBodyBytes = 1 << 20

// codeStore is the consumer-side offer-code store the redeem handler needs.
type codeStore interface {
	Get(ctx context.Context, canonical string) (OfferCode, error)
	Save(ctx context.Context, c OfferCode) error
}

// profileStore is the consumer-side profile store: the redeem path loads the
// caller's profile, activates the granted tier, and saves it back.
type profileStore interface {
	Get(ctx context.Context, userID string) (*profiles.UserProfile, error)
	Save(ctx context.Context, p *profiles.UserProfile) error
}

// tierSync mirrors the .NET IAuth0ManagementClient subset used here: push the
// new tier into Auth0's app_metadata. profiles.Auth0Manager satisfies it.
type tierSync interface {
	UpdateSubscriptionTier(ctx context.Context, userID, tier string) error
}

type handler struct {
	codes    codeStore
	profiles profileStore
	auth0    tierSync
	now      func() time.Time
	logger   *slog.Logger
}

// Routes registers the authed redeem endpoint on mux.
func Routes(mux *http.ServeMux, codes codeStore, profileStore profileStore, auth0 tierSync, now func() time.Time, logger *slog.Logger) {
	h := &handler{codes: codes, profiles: profileStore, auth0: auth0, now: now, logger: logger}
	mux.HandleFunc("POST /v1/offer-codes/redeem", h.redeem)
}

type redeemRequest struct {
	Code string `json:"code"`
}

// redeemResponse mirrors the .NET RedeemOfferCodeResponse: { tier, expiresAt }.
type redeemResponse struct {
	Tier      string              `json:"tier"`
	ExpiresAt platform.DotNetTime `json:"expiresAt"`
}

// apiErrorResponse mirrors the .NET ApiErrorResponse { error, message }. Unlike
// the bodyless-backfill paths, the offer-code errors carry a human message.
type apiErrorResponse struct {
	Error   string  `json:"error"`
	Message *string `json:"message"`
}

// redeem implements POST /v1/offer-codes/redeem. It mirrors the .NET endpoint's
// error contract exactly: malformed code -> 400 invalid_code_format, unknown
// code -> 404 invalid_code, already-claimed -> 409 code_already_redeemed,
// caller already paid -> 409 already_subscribed.
func (h *handler) redeem(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req redeemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	canonical, err := Normalize(req.Code)
	if err != nil {
		var fe *InvalidFormatError
		if errors.As(err, &fe) {
			h.writeError(r, w, http.StatusBadRequest, "invalid_code_format", fe.Message)
			return
		}
		h.serverError(w, r, "normalize offer code", err)
		return
	}

	code, err := h.codes.Get(r.Context(), canonical)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			h.writeError(r, w, http.StatusNotFound, "invalid_code", "Offer code '"+canonical+"' was not found.")
			return
		}
		h.serverError(w, r, "load offer code", err)
		return
	}
	if code.IsRedeemed() {
		h.writeError(r, w, http.StatusConflict, "code_already_redeemed", "Offer code '"+canonical+"' has already been redeemed.")
		return
	}

	// A missing profile for an authenticated caller is a server-side
	// inconsistency: .NET throws UserProfileNotFoundException, which the endpoint
	// does not catch, yielding a 500. Mirror that — any load failure is a 500.
	profile, err := h.profiles.Get(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "load profile", err)
		return
	}
	if profile.Tier != profiles.TierFree {
		h.writeError(r, w, http.StatusConflict, "already_subscribed",
			"User already has an active subscription; offer codes are only available to free-tier users.")
		return
	}

	now := h.now()
	if err := code.Redeem(userID, now); err != nil {
		h.serverError(w, r, "redeem offer code", err)
		return
	}
	expiry := now.AddDate(0, 0, code.DurationDays)
	profile.ActivateSubscription(code.Tier, expiry)

	if err := h.codes.Save(r.Context(), code); err != nil {
		h.serverError(w, r, "save offer code", err)
		return
	}
	if err := h.profiles.Save(r.Context(), profile); err != nil {
		h.serverError(w, r, "save profile", err)
		return
	}
	if err := h.auth0.UpdateSubscriptionTier(r.Context(), profile.UserID, profile.Tier.String()); err != nil {
		h.serverError(w, r, "sync auth0 tier", err)
		return
	}

	h.writeJSON(r, w, redeemResponse{Tier: profile.Tier.String(), ExpiresAt: platform.DotNetTime(*profile.SubscriptionExpiry)})
}

func (h *handler) writeJSON(r *http.Request, w http.ResponseWriter, v any) {
	body, err := encodeJSON(v)
	if err != nil {
		h.serverError(w, r, "encode response", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write offer-code response", "error", err)
	}
}

func (h *handler) writeError(r *http.Request, w http.ResponseWriter, status int, code, message string) {
	msg := message
	body, err := encodeJSON(apiErrorResponse{Error: code, Message: &msg})
	if err != nil {
		h.serverError(w, r, "encode error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write offer-code error body", "error", err)
	}
}

func encodeJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func (h *handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.ErrorContext(r.Context(), "offer-code request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
