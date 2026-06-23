package profiles

import (
	"context"
	"errors"
	"time"
)

// activityWriteDedupeWindow is the 24h deduplication window for activity writes:
// activity is persisted at most once per 24h to avoid an upsert per request.
const activityWriteDedupeWindow = 24 * time.Hour

// ActivityRecorder updates a profile's LastActiveAt, deduping writes within 24h.
// It adapts the profile store to the middleware's activityRecorder interface:
// read the profile, skip when it was active within the window, otherwise advance
// and save. An unknown user is a no-op (registration is POST /v1/me only).
type ActivityRecorder struct {
	store profileStore
}

// NewActivityRecorder builds the recorder over the given profile store.
func NewActivityRecorder(store profileStore) *ActivityRecorder {
	return &ActivityRecorder{store: store}
}

// RecordActivity advances the user's LastActiveAt to now, persisting only when
// more than 24h has elapsed since the last recorded activity.
func (r *ActivityRecorder) RecordActivity(ctx context.Context, userID string, now time.Time) error {
	profile, err := r.store.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	if now.Sub(profile.LastActiveAt) < activityWriteDedupeWindow {
		return nil
	}
	profile.RecordActivity(now)
	return r.store.Save(ctx, profile)
}

// TierLookup answers the rate-limiter's paid/free question from the Cosmos
// profile tier — the authoritative entitlement source per ADR 0010, never the
// JWT claim. A user with no profile is treated as free. The injected clock lets
// the lazy expiry check (EffectiveTier) collapse a lapsed paid tier to free.
type TierLookup struct {
	store profileStore
	now   func() time.Time
}

// NewTierLookup builds the lookup over the given profile store and clock.
func NewTierLookup(store profileStore, now func() time.Time) *TierLookup {
	return &TierLookup{store: store, now: now}
}

// IsPaidUser reports whether the user's effective tier is paid (anything but
// Free) at the current instant — a paid tier whose subscription has lapsed (with
// no live grace period) reads as free. A missing profile yields false (free)
// without error.
func (l *TierLookup) IsPaidUser(ctx context.Context, userID string) (bool, error) {
	profile, err := l.store.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	return profile.EffectiveTier(l.now()).IsPaid(), nil
}
