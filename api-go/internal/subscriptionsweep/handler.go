// Package subscriptionsweep holds the subscription-sweep worker mode
// (WORKER_MODE=subscription-sweep): once a day it finds paid profiles whose
// entitlement has lapsed — offer-code grants past their duration, or App Store
// subscriptions past expiry whose webhook never arrived — and reverts their
// stored tier to Free, then syncs Auth0's subscription_tier metadata to match.
//
// The read path (UserProfile.EffectiveTier) already makes those users Free on
// every entitlement gate the moment they lapse, so this sweep is hygiene and a
// transient Auth0 sync failure here is benign.
package subscriptionsweep

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptionevents"
)

// Finder returns the lapsed-paid set: every profile whose stored tier is paid but
// whose EffectiveTier(now) has collapsed to Free.
type Finder interface {
	LapsedPaid(ctx context.Context, now time.Time) ([]*profiles.UserProfile, error)
}

// Saver persists a downgraded profile.
type Saver interface {
	Save(ctx context.Context, p *profiles.UserProfile) error
}

// Auth0Syncer keeps Auth0's app_metadata.subscription_tier in step with the
// downgrade.
type Auth0Syncer interface {
	UpdateSubscriptionTier(ctx context.Context, userID, tier string) error
}

// Notifier is told about each successful downgrade save.
type Notifier interface {
	Changed(ctx context.Context, before, after subscriptionevents.Snapshot, source subscriptionevents.Source)
}

// Handler runs one subscription-sweep cycle.
type Handler struct {
	finder Finder
	saver  Saver
	auth0  Auth0Syncer
	notify Notifier
	logger *slog.Logger
	now    func() time.Time
}

// New builds a subscription-sweep handler. now supplies the lapsed cutoff.
func New(finder Finder, saver Saver, auth0 Auth0Syncer, notify Notifier, logger *slog.Logger, now func() time.Time) *Handler {
	return &Handler{finder: finder, saver: saver, auth0: auth0, notify: notify, logger: logger, now: now}
}

// Run executes one sweep cycle and returns the number of profiles fully
// downgraded. A scan failure is returned; a per-profile Save or Auth0 failure
// is logged and skipped, and the next daily run retries that profile.
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

// downgrade saves before syncing Auth0: a failed save leaves stored state
// untouched, and a failed sync only leaves informational metadata stale
// because entitlements gate on EffectiveTier.
func (h *Handler) downgrade(ctx context.Context, p *profiles.UserProfile) error {
	before := subscriptionevents.SnapshotOf(p)
	p.ExpireSubscription()
	if err := h.saver.Save(ctx, p); err != nil {
		return fmt.Errorf("save downgraded profile %q: %w", p.UserID, err)
	}
	if h.notify != nil {
		h.notify.Changed(ctx, before, subscriptionevents.SnapshotOf(p), subscriptionevents.SourceSweep)
	}
	if err := h.auth0.UpdateSubscriptionTier(ctx, p.UserID, p.Tier.String()); err != nil {
		return fmt.Errorf("sync auth0 tier for %q: %w", p.UserID, err)
	}
	return nil
}
