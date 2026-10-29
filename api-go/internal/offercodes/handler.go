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
	"github.com/AmyDe/town-crier/api-go/internal/subscriptionevents"
)

const maxBodyBytes = 1 << 20

var errGrantNotHealable = errors.New("offer-code grant cannot be re-applied")

// codeStore is the consumer-side offer-code store the redeem handler needs.
type codeStore interface {
	Get(ctx context.Context, canonical string) (OfferCode, error)
	Save(ctx context.Context, c OfferCode) error
	// RedeemWithCAS atomically redeems the code for userID at now. Returns
	// ErrNotFound if the code doesn't exist, ErrAlreadyRedeemed if every
	// redemption slot is already consumed, or ErrAlreadyRedeemedByUser if
	// userID has already personally redeemed this code.
	RedeemWithCAS(ctx context.Context, canonical, userID string, now time.Time) error
	// RedemptionTime returns when userID redeemed the code, or ErrNotFound if
	// userID has no redemption of it.
	RedemptionTime(ctx context.Context, canonical, userID string) (time.Time, error)
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

// subscriptionNotifier is told about each successful tier-changing save.
type subscriptionNotifier interface {
	Changed(ctx context.Context, before, after subscriptionevents.Snapshot, source subscriptionevents.Source)
}

type handler struct {
	codes    codeStore
	profiles profileStore
	auth0    tierSync
	notifier subscriptionNotifier
	now      func() time.Time
	logger   *slog.Logger
}

// Routes registers the authed redeem endpoint on mux.
func Routes(mux *http.ServeMux, codes codeStore, profileStore profileStore, auth0 tierSync, notifier subscriptionNotifier, now func() time.Time, logger *slog.Logger) {
	h := &handler{codes: codes, profiles: profileStore, auth0: auth0, notifier: notifier, now: now, logger: logger}
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

// apiErrorResponse is the error response body { error, message }.
type apiErrorResponse struct {
	Error   string  `json:"error"`
	Message *string `json:"message"`
}

// redeem implements POST /v1/offer-codes/redeem: malformed code -> 400
// invalid_code_format, unknown code -> 404 invalid_code, already claimed by
// another user -> 409 code_already_redeemed, caller already paid -> 409
// already_subscribed. A caller who already redeemed the code gets the original
// grant re-applied idempotently and the same success response.
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

	redeemedAt, err := h.codes.RedemptionTime(r.Context(), canonical, userID)
	switch {
	case err == nil:
		h.heal(w, r, userID, code, redeemedAt)
		return
	case !errors.Is(err, ErrNotFound):
		h.serverError(w, r, "load redemption time", err)
		return
	}

	if code.IsFullyRedeemed() {
		h.writeError(r, w, http.StatusConflict, "code_already_redeemed", "Offer code '"+canonical+"' has already been redeemed.")
		return
	}

	profile, err := h.profiles.Get(r.Context(), userID)
	if err != nil {
		h.serverError(w, r, "load profile", err)
		return
	}

	now := h.now().Truncate(time.Microsecond)

	// Gate on the effective tier, not the stored one: a lapsed paid grant is
	// entitled to Free and may redeem a new code.
	if profile.EffectiveTier(now).IsPaid() {
		h.writeError(r, w, http.StatusConflict, "already_subscribed",
			"User already has an active subscription; offer codes are only available to free-tier users.")
		return
	}

	switch redeemErr := h.codes.RedeemWithCAS(r.Context(), canonical, userID, now); {
	case redeemErr == nil:
	case errors.Is(redeemErr, ErrNotFound):
		h.writeError(r, w, http.StatusNotFound, "invalid_code", "Offer code '"+canonical+"' was not found.")
		return
	case errors.Is(redeemErr, ErrAlreadyRedeemedByUser):
		at, err := h.codes.RedemptionTime(r.Context(), canonical, userID)
		if err != nil {
			h.serverError(w, r, "load redemption time", err)
			return
		}
		h.heal(w, r, userID, code, at)
		return
	case errors.Is(redeemErr, ErrAlreadyRedeemed):
		h.writeError(r, w, http.StatusConflict, "code_already_redeemed", "Offer code '"+canonical+"' has already been redeemed.")
		return
	default:
		h.serverError(w, r, "redeem offer code", redeemErr)
		return
	}

	// Re-read so the profile grant uses the authoritative stored tier and duration.
	code, err = h.codes.Get(r.Context(), canonical)
	if err != nil {
		h.serverError(w, r, "reload offer code after redeem", err)
		return
	}

	expiry := now.AddDate(0, 0, code.DurationDays)
	var before subscriptionevents.Snapshot
	err = profiles.RetryOnConflict(r.Context(), profiles.DefaultSaveAttempts, func(ctx context.Context) error {
		current, err := h.profiles.Get(ctx, userID)
		if err != nil {
			return err
		}
		before = subscriptionevents.SnapshotOf(current)
		current.ActivateSubscription(code.Tier, expiry)
		profile = current
		return h.profiles.Save(ctx, current)
	})
	if err != nil {
		h.serverError(w, r, "save profile", err)
		return
	}
	if h.notifier != nil {
		h.notifier.Changed(r.Context(), before, subscriptionevents.SnapshotOf(profile), subscriptionevents.SourceOfferCode)
	}
	h.syncAuth0(r.Context(), profile)

	h.writeJSON(r, w, redeemResponse{Tier: profile.Tier.String(), ExpiresAt: platform.DotNetTime(*profile.SubscriptionExpiry)})
}

// heal re-applies a grant the caller already redeemed, for a retry after a
// partial failure. The expiry derives from the original redemption time, and
// the profile is never shortened or downgraded. A lapsed grant, or a caller
// now on another paid plan, gets code_already_redeemed. No notification is sent.
func (h *handler) heal(w http.ResponseWriter, r *http.Request, userID string, code OfferCode, redeemedAt time.Time) {
	expiry := redeemedAt.AddDate(0, 0, code.DurationDays)
	now := h.now().Truncate(time.Microsecond)
	if !expiry.After(now) {
		h.writeError(r, w, http.StatusConflict, "code_already_redeemed", "Offer code '"+code.Code+"' has already been redeemed.")
		return
	}
	var profile *profiles.UserProfile
	err := profiles.RetryOnConflict(r.Context(), profiles.DefaultSaveAttempts, func(ctx context.Context) error {
		current, err := h.profiles.Get(ctx, userID)
		if err != nil {
			return err
		}
		profile = current
		if current.Tier > code.Tier || (current.Tier == code.Tier && current.SubscriptionExpiry != nil && !current.SubscriptionExpiry.Before(expiry)) {
			return nil
		}
		if current.EffectiveTier(now).IsPaid() {
			return errGrantNotHealable
		}
		current.ActivateSubscription(code.Tier, expiry)
		return h.profiles.Save(ctx, current)
	})
	if errors.Is(err, errGrantNotHealable) {
		h.writeError(r, w, http.StatusConflict, "code_already_redeemed", "Offer code '"+code.Code+"' has already been redeemed.")
		return
	}
	if err != nil {
		h.serverError(w, r, "save profile", err)
		return
	}
	h.syncAuth0(r.Context(), profile)

	respExpiry := expiry
	if profile.SubscriptionExpiry != nil {
		respExpiry = *profile.SubscriptionExpiry
	}
	h.writeJSON(r, w, redeemResponse{Tier: profile.Tier.String(), ExpiresAt: platform.DotNetTime(respExpiry)})
}

// syncAuth0 pushes the tier to Auth0. The database is the authority, so a
// failure is logged and does not fail the request.
func (h *handler) syncAuth0(ctx context.Context, p *profiles.UserProfile) {
	if err := h.auth0.UpdateSubscriptionTier(ctx, p.UserID, p.Tier.String()); err != nil {
		h.logger.WarnContext(ctx, "offer-code auth0 tier sync failed", "error", err)
	}
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
	body, err := httputil.EncodeJSON(apiErrorResponse{Error: code, Message: &message})
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
