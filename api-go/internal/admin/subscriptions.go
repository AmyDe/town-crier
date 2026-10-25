package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptionevents"
)

// farFutureExpiry is the sentinel expiry an admin grant assigns to a paid tier
// (2099-12-31T00:00:00+00:00).
var farFutureExpiry = time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)

type grantRequest struct {
	Email string `json:"email"`
	Tier  string `json:"tier"`
}

// grantResult is the response shape for PUT /v1/admin/subscriptions: { userId, email, tier }.
type grantResult struct {
	UserID string  `json:"userId"`
	Email  *string `json:"email"`
	Tier   string  `json:"tier"`
}

// grantSubscription implements PUT /v1/admin/subscriptions: set a user's tier by
// email. Granting Free expires the subscription; any paid tier activates it to
// the far-future expiry. A missing profile is a bodyless 404.
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

	var (
		profile *profiles.UserProfile
		before  subscriptionevents.Snapshot
		stage   string
	)
	err = profiles.RetryOnConflict(r.Context(), profiles.DefaultSaveAttempts, func(ctx context.Context) error {
		stage = "load profile by email"
		current, err := h.profiles.GetByEmail(ctx, req.Email)
		if err != nil {
			return err
		}
		before = subscriptionevents.SnapshotOf(current)
		if tier == profiles.TierFree {
			current.ExpireSubscription()
		} else {
			current.ActivateSubscription(tier, farFutureExpiry)
		}
		profile = current

		// Sync Auth0 before writing Postgres: with no transaction, this order means a
		// failed sync leaves no committed tier change behind.
		stage = "sync auth0 tier"
		if err := h.auth0.UpdateSubscriptionTier(ctx, current.UserID, current.Tier.String()); err != nil {
			return err
		}
		stage = "save profile"
		return h.profiles.Save(ctx, current)
	})
	if err != nil {
		if errors.Is(err, profiles.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.serverError(w, r, stage, err)
		return
	}
	if h.notifier != nil {
		h.notifier.Changed(r.Context(), before, subscriptionevents.SnapshotOf(profile), subscriptionevents.SourceAdmin)
	}

	h.writeJSON(r, w, grantResult{UserID: profile.UserID, Email: profile.Email, Tier: profile.Tier.String()})
}
