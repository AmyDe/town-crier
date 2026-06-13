package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// farFutureExpiry is the sentinel expiry an admin grant assigns to a paid tier,
// mirroring the .NET GrantSubscriptionCommandHandler.FarFutureExpiry
// (2099-12-31T00:00:00+00:00).
var farFutureExpiry = time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)

type grantRequest struct {
	Email string `json:"email"`
	Tier  string `json:"tier"`
}

// grantResult mirrors the .NET GrantSubscriptionResult: { userId, email, tier }.
type grantResult struct {
	UserID string  `json:"userId"`
	Email  *string `json:"email"`
	Tier   string  `json:"tier"`
}

// grantSubscription implements PUT /v1/admin/subscriptions: set a user's tier by
// email. Granting Free expires the subscription; any paid tier activates it to
// the far-future expiry. A missing profile is a bodyless 404, matching .NET's
// Results.NotFound() on UserProfileNotFoundException.
func (h *handler) grantSubscription(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req grantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	tier, err := profiles.ParseSubscriptionTier(req.Tier)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	profile, err := h.profiles.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, profiles.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, "load profile by email", err)
		return
	}

	if tier == profiles.TierFree {
		profile.ExpireSubscription()
	} else {
		profile.ActivateSubscription(tier, farFutureExpiry)
	}

	if err := h.profiles.Save(r.Context(), profile); err != nil {
		h.serverError(w, r, "save profile", err)
		return
	}
	if err := h.auth0.UpdateSubscriptionTier(r.Context(), profile.UserID, profile.Tier.String()); err != nil {
		h.serverError(w, r, "sync auth0 tier", err)
		return
	}

	h.writeJSON(r, w, grantResult{UserID: profile.UserID, Email: profile.Email, Tier: profile.Tier.String()})
}
