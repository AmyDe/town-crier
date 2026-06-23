// Package subscriptionsweep holds the subscription-sweep worker mode
// (WORKER_MODE=subscription-sweep): once a day it finds paid profiles whose
// entitlement has lapsed — offer-code grants past their duration, or App Store
// subscriptions past expiry whose webhook never arrived — and reverts their
// stored Cosmos tier to Free, then syncs Auth0's subscription_tier metadata to
// match.
//
// It is the stored-state reconciliation half of GH #608 (Option B). The lazy
// read-path check (UserProfile.EffectiveTier, Phase 1) already makes those users
// Free on every entitlement gate the moment they lapse; this sweep is hygiene
// that keeps the persisted Cosmos document and the Auth0 metadata clean. Because
// the read path is already correct, a transient Auth0 sync failure here is benign
// — entitlements gate on Cosmos + EffectiveTier, never on the Auth0 metadata.
//
// It mirrors internal/dormant: consumer-side interfaces declared here, concrete
// stores injected from main(), hand-written test fakes, an injected now for a
// pinned clock, and per-item failure isolation so one bad profile never aborts
// the cycle.
package subscriptionsweep

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// Finder returns the lapsed-paid set: every profile whose stored tier is paid but
// whose EffectiveTier(now) has collapsed to Free. profiles.AdminStore satisfies
// it via LapsedPaid.
type Finder interface {
	LapsedPaid(ctx context.Context, now time.Time) ([]*profiles.UserProfile, error)
}

// Saver persists a downgraded profile back to Cosmos. profiles.AdminStore
// satisfies it via Save (an upsert keyed on the user id / partition key).
type Saver interface {
	Save(ctx context.Context, p *profiles.UserProfile) error
}

// Auth0Syncer keeps Auth0's app_metadata.subscription_tier in step with the
// downgrade. profiles.Auth0Client (and the NoOpAuth0Client fallback) satisfy it
// via UpdateSubscriptionTier — the same call the redemption and verify paths use.
type Auth0Syncer interface {
	UpdateSubscriptionTier(ctx context.Context, userID, tier string) error
}

// Handler runs one subscription-sweep cycle.
type Handler struct {
	finder Finder
	saver  Saver
	auth0  Auth0Syncer
	logger *slog.Logger
	now    func() time.Time
}

// New builds a subscription-sweep handler. now is injected so tests pin the
// lapsed cutoff; production passes time.Now.
func New(finder Finder, saver Saver, auth0 Auth0Syncer, logger *slog.Logger, now func() time.Time) *Handler {
	return &Handler{finder: finder, saver: saver, auth0: auth0, logger: logger, now: now}
}

// Run executes one sweep cycle and returns the number of profiles fully
// downgraded. It scans for lapsed paid profiles, then for each reverts the stored
// tier to Free, saves, and syncs Auth0. A scan failure is fatal to the cycle; a
// per-profile Save or Auth0 failure is logged and skipped (the profile is not
// counted and the next daily run retries it) and the cycle continues — mirroring
// dormant.Handler.Run.
func (h *Handler) Run(ctx context.Context) (int, error) {
	now := h.now()
	lapsed, err := h.finder.LapsedPaid(ctx, now)
	if err != nil {
		return 0, fmt.Errorf("find lapsed paid profiles: %w", err)
	}

	downgraded := 0
	for _, p := range lapsed {
		if err := h.downgrade(ctx, p); err != nil {
			h.logger.ErrorContext(ctx, "subscription downgrade failed; will retry next cycle", "user", p.UserID, "error", err)
			continue
		}
		downgraded++
	}
	h.logger.InfoContext(ctx, "subscription sweep cycle complete", "scanned", len(lapsed), "downgraded", downgraded)
	return downgraded, nil
}

// downgrade reverts one lapsed profile to the Free tier: ExpireSubscription
// clears the stored tier/expiry/grace, Save persists it, then the Auth0 metadata
// is synced. Cosmos is written before Auth0 (mirroring the erasure cascade): if
// Save fails the stored state is untouched and the next cycle retries; if Auth0
// fails after a successful Save the stored tier is already Free, so the read path
// is correct and only the informational Auth0 metadata is momentarily stale.
func (h *Handler) downgrade(ctx context.Context, p *profiles.UserProfile) error {
	p.ExpireSubscription()
	if err := h.saver.Save(ctx, p); err != nil {
		return fmt.Errorf("save downgraded profile %q: %w", p.UserID, err)
	}
	if err := h.auth0.UpdateSubscriptionTier(ctx, p.UserID, profiles.TierFree.String()); err != nil {
		return fmt.Errorf("sync auth0 tier for %q: %w", p.UserID, err)
	}
	return nil
}
