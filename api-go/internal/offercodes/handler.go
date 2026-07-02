package offercodes

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/httputil"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// maxCASRetries is the maximum number of etag-conditional replace attempts for
// the redemption CAS loop. After this many ErrCASPreconditionFailed responses
// the handler treats the code as already redeemed (the overwhelmingly likely
// cause is a concurrent winner).
const maxCASRetries = 3

const maxBodyBytes = 1 << 20

// codeStore is the consumer-side offer-code store the redeem handler needs.
type codeStore interface {
	Get(ctx context.Context, canonical string) (OfferCode, error)
	Save(ctx context.Context, c OfferCode) error
	// RedeemWithCAS atomically redeems the code using an etag-conditional replace.
	// Returns ErrNotFound, ErrAlreadyRedeemed, or platform.ErrCASPreconditionFailed
	// (etag mismatch — lost the CAS race to a concurrent writer).
	RedeemWithCAS(ctx context.Context, canonical, userID string, now time.Time) error
}

// profileStore is the consumer-side profile store: the redeem path loads the
// caller's profile, activates the granted tier, and saves it back.
type profileStore interface {
	Get(ctx context.Context, userID string) (*profiles.UserProfile, error)
	Save(ctx context.Context, p *profiles.UserProfile) error
}

// tierSync is the Auth0 Management client subset used here: push the new tier
// into Auth0's app_metadata. profiles.Auth0Manager satisfies it.
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

// redeemResponse is the POST /v1/offer-codes/redeem success body: { tier, expiresAt }.
type redeemResponse struct {
	Tier      string              `json:"tier"`
	ExpiresAt platform.DotNetTime `json:"expiresAt"`
}

// apiErrorResponse is the error response body { error, message }. Unlike
// the bodyless-backfill paths, the offer-code errors carry a human message.
type apiErrorResponse struct {
	Error   string  `json:"error"`
	Message *string `json:"message"`
}

// redeem implements POST /v1/offer-codes/redeem: malformed code -> 400
// invalid_code_format, unknown code -> 404 invalid_code, already-claimed ->
// 409 code_already_redeemed, caller already paid -> 409 already_subscribed.
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

	// Verify the code exists and is not already redeemed before loading the
	// profile. This is a fast-path check; the CAS loop below is the authoritative
	// single-use enforcement.
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
	// inconsistency; any load failure is a 500.
	profile, err := h.profiles.Get(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "load profile", err)
		return
	}

	now := h.now()

	// Gate on the EFFECTIVE tier, not the raw stored one: a lapsed paid grant
	// (Tier still Pro/Personal but SubscriptionExpiry — and any grace period —
	// in the past) is entitled to Free, so it must be allowed to redeem a new
	// code. Only a currently-active paid tier blocks redemption.
	if profile.EffectiveTier(now).IsPaid() {
		h.writeError(r, w, http.StatusConflict, "already_subscribed",
			"User already has an active subscription; offer codes are only available to free-tier users.")
		return
	}

	// CAS retry loop: RedeemWithCAS does read→redeem-in-memory→etag-conditional
	// replace, making the redemption atomic. On ErrCASPreconditionFailed (412 — a
	// concurrent writer mutated the document first) we re-read to decide whether
	// to retry or surface 409.
	var redeemErr error
	for range maxCASRetries {
		redeemErr = h.codes.RedeemWithCAS(r.Context(), canonical, userID, now)
		if redeemErr == nil {
			break
		}
		if !errors.Is(redeemErr, platform.ErrCASPreconditionFailed) {
			break
		}
		// Lost the CAS race. Re-read the code to determine its current state.
		reread, rerr := h.codes.Get(r.Context(), canonical)
		if rerr != nil {
			h.serverError(w, r, "re-read offer code after CAS conflict", rerr)
			return
		}
		if reread.IsRedeemed() {
			// Another request won — treat as already redeemed.
			h.writeError(r, w, http.StatusConflict, "code_already_redeemed", "Offer code '"+canonical+"' has already been redeemed.")
			return
		}
		// Code still available after conflict (very rare); loop to retry.
	}
	switch {
	case redeemErr == nil:
		// success — fall through
	case errors.Is(redeemErr, ErrAlreadyRedeemed):
		h.writeError(r, w, http.StatusConflict, "code_already_redeemed", "Offer code '"+canonical+"' has already been redeemed.")
		return
	case errors.Is(redeemErr, platform.ErrCASPreconditionFailed):
		// Exhausted retries without winning; treat conservatively as already redeemed.
		h.writeError(r, w, http.StatusConflict, "code_already_redeemed", "Offer code '"+canonical+"' has already been redeemed.")
		return
	default:
		h.serverError(w, r, "redeem offer code", redeemErr)
		return
	}

	// RedeemWithCAS persisted the redeemed code atomically; re-read for the tier
	// and duration so the profile grant is based on the authoritative stored state.
	code, err = h.codes.Get(r.Context(), canonical)
	if err != nil {
		h.serverError(w, r, "reload offer code after redeem", err)
		return
	}

	expiry := now.AddDate(0, 0, code.DurationDays)
	profile.ActivateSubscription(code.Tier, expiry)

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
	body, err := httputil.EncodeJSON(v)
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
	body, err := httputil.EncodeJSON(apiErrorResponse{Error: code, Message: &msg})
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

func (h *handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.ErrorContext(r.Context(), "offer-code request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
