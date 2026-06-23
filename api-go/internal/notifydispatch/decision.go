package notifydispatch

import (
	"context"
	"log/slog"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// zoneMatcher runs the cross-partition point-in-zone lookup.
// *watchzones.CosmosStore satisfies it.
type zoneMatcher interface {
	FindZonesContaining(ctx context.Context, latitude, longitude float64) ([]watchzones.WatchZone, error)
}

// savedMatcher runs the cross-partition saved-bookmark lookup.
// *savedapplications.CosmosStore satisfies it.
type savedMatcher interface {
	UserIDsForApplication(ctx context.Context, applicationUID string, authorityID int) ([]string, error)
}

// DecisionDispatcher handles decision-event fan-out: when a polled application
// transitions into a decision state, it computes the union of users matched by a
// watch zone (geographic) and users who bookmarked the application (saved), then
// dispatches exactly one DecisionUpdate notification per user. Idempotent — at
// most one DecisionUpdate per (userId, applicationUid, authorityId).
type DecisionDispatcher struct {
	notifications notificationWriter
	zones         zoneMatcher
	saved         savedMatcher
	profiles      profileReader
	devices       deviceReader
	state         stateReader
	push          pushSender
	newID         func() string
	now           func() time.Time
	logger        *slog.Logger
	metrics       notificationMetricsRecorder
}

// WithMetrics wires the recorder the dispatcher records
// towncrier.notifications.created on. A post-construction setter so the existing
// dispatch call sites and tests are unaffected; cmd/worker calls it once after
// building the dispatcher. Returns the dispatcher for chaining.
func (d *DecisionDispatcher) WithMetrics(rec notificationMetricsRecorder) *DecisionDispatcher {
	d.metrics = rec
	return d
}

// NewDecisionDispatcher wires the decision dispatcher. newID mints the
// notification id; now stamps the record's creation time. Both are injected so
// tests can pin them.
func NewDecisionDispatcher(
	notifs notificationWriter,
	zones zoneMatcher,
	saved savedMatcher,
	profs profileReader,
	devices deviceReader,
	state stateReader,
	push pushSender,
	newID func() string,
	now func() time.Time,
	logger *slog.Logger,
) *DecisionDispatcher {
	return &DecisionDispatcher{
		notifications: notifs,
		zones:         zones,
		saved:         saved,
		profiles:      profs,
		devices:       devices,
		state:         state,
		push:          push,
		newID:         newID,
		now:           now,
		logger:        logger,
	}
}

// userMatch accumulates which sources matched a user and the first zone id to
// attribute the notification to (saved-only matches leave it nil).
type userMatch struct {
	zone        bool
	saved       bool
	watchZoneID *string
}

// Dispatch fans a decision event out to every matched user. It is idempotent per
// (user, application, authority): a re-dispatch finds the existing DecisionUpdate
// and skips.
func (d *DecisionDispatcher) Dispatch(ctx context.Context, app applications.PlanningApplication) error {
	matches := map[string]*userMatch{}

	// Zone matchers — only meaningful when the application has coordinates.
	if app.Latitude != nil && app.Longitude != nil {
		zones, err := d.zones.FindZonesContaining(ctx, *app.Latitude, *app.Longitude)
		if err != nil {
			return err
		}
		for _, zone := range zones {
			m := matches[zone.UserID]
			if m == nil {
				m = &userMatch{}
				matches[zone.UserID] = m
			}
			m.zone = true
			if m.watchZoneID == nil {
				id := zone.ID
				m.watchZoneID = &id
			}
		}
	}

	// Saved bookmark holders — scoped to the polled application's council, since
	// PlanIt uids collide across councils (tc-th98 / GH#384).
	savedUserIDs, err := d.saved.UserIDsForApplication(ctx, app.UID, app.AreaID)
	if err != nil {
		return err
	}
	for _, userID := range savedUserIDs {
		m := matches[userID]
		if m == nil {
			m = &userMatch{}
			matches[userID] = m
		}
		m.saved = true
	}

	for userID, m := range matches {
		if err := d.dispatchForUser(ctx, app, userID, m); err != nil {
			return err
		}
	}
	return nil
}

// dispatchForUser dispatches the decision event to one matched user, gated on
// the per-user idempotency read. The push fires only when the user is on a paid
// tier with push enabled AND opted into decision pushes on a matching source
// (zone DecisionPush or saved SavedDecisionPush). The record is always written.
func (d *DecisionDispatcher) dispatchForUser(ctx context.Context, app applications.PlanningApplication, userID string, m *userMatch) error {
	existing, err := d.notifications.GetByUserAndApplication(ctx, userID, app.UID, app.AreaID, notifications.EventDecisionUpdate)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	profile, err := d.profiles.Get(ctx, userID)
	if err != nil {
		d.logger.WarnContext(ctx, "decision: profile unavailable, skipping", "user", userID, "error", err)
		return nil //nolint:nilerr // a missing profile is a skip, not a fan-out failure
	}
	if profile == nil {
		return nil
	}

	n := newRecord(recordInput{
		id:          d.newID(),
		userID:      userID,
		app:         app,
		watchZoneID: m.watchZoneID,
		eventType:   notifications.EventDecisionUpdate,
		sources:     mergeSources(m),
		now:         d.now().UTC(),
	})

	if d.canPush(profile, m) {
		n.PushSent = sendInstantPush(ctx, instantPushDeps{
			devices: d.devices,
			state:   d.state,
			push:    d.push,
			logger:  d.logger,
		}, userID, n)
	}

	if err := d.notifications.Create(ctx, n); err != nil {
		return err
	}
	if d.metrics != nil {
		d.metrics.NotificationCreated(ctx, string(n.EventType), n.Sources)
	}
	return nil
}

// canPush OR-merges the per-channel decision-push toggles across the matching
// sources, gated on the paid tier and the global push preference.
func (d *DecisionDispatcher) canPush(profile *profiles.UserProfile, m *userMatch) bool {
	// A paid tier whose subscription has lapsed (EffectiveTier) reads as Free and
	// is not entitled to an instant push.
	if !profile.EffectiveTier(d.now()).IsPaid() || !profile.Preferences.PushEnabled {
		return false
	}
	zonePushOptIn := m.zone && m.watchZoneID != nil && profile.ZonePreferences[*m.watchZoneID].DecisionPush
	savedPushOptIn := m.saved && profile.Preferences.SavedDecisionPush
	return zonePushOptIn || savedPushOptIn
}

// mergeSources renders the NotificationSources flag string for a match:
// "Zone", "Saved", or "Zone,Saved" when both apply.
func mergeSources(m *userMatch) string {
	switch {
	case m.zone && m.saved:
		return sourceZone + "," + sourceSaved
	case m.saved:
		return sourceSaved
	default:
		return sourceZone
	}
}
