// Package notifydispatch is the poll-path notification fan-out: the per-app
// orchestration the poll-sb handler runs after upserting a changed planning
// application. It implements two dispatch paths: the new-application zone
// fan-out (Enqueuer) and the non-decision → decision transition fan-out
// (DecisionDispatcher).
//
// It lives in its own package, not in notifications, to avoid an import cycle:
// the watchzones package already imports notifications (for the latest-unread
// projection), and the dispatchers depend on watchzones, profiles,
// devicetokens, notificationstate, savedapplications and the notifications
// store. Keeping notifications a leaf store package and composing here is the
// idiomatic Go resolution.
//
// Idempotency is the load-bearing property: every dispatch is gated on a
// (userId, applicationUid, authorityId, eventType) dedup read, so a re-poll of
// an unchanged application never double-notifies. Free-tier users get the
// notification record (which feeds the weekly digest) but no instant push —
// matching the server-enforced tier entitlement.
package notifydispatch

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// notificationWriter is the consumer-side slice of the Notifications store the
// dispatchers need: a dedup read and a create. *notifications.DigestStore
// satisfies it.
type notificationWriter interface {
	GetByUserAndApplication(ctx context.Context, userID, applicationUID string, authorityID int, eventType notifications.EventType) (*notifications.DigestNotification, error)
	Create(ctx context.Context, n notifications.DigestNotification) error
}

// profileReader loads a user's profile to read the tier (instant push is
// paid-only) and the push preference. *profiles.CosmosStore satisfies it.
type profileReader interface {
	Get(ctx context.Context, userID string) (*profiles.UserProfile, error)
}

// deviceReader lists a user's device tokens for a push and prunes the ones APNs
// reports permanently invalid. *devicetokens.CosmosStore satisfies it.
type deviceReader interface {
	ListByUser(ctx context.Context, userID string) ([]devicetokens.DeviceRegistration, error)
	Delete(ctx context.Context, userID, token string) error
}

// stateReader supplies the unread-badge input: the total unread tally
// (read_at IS NULL, ADR 0035). *notificationstate.PostgresStore satisfies it.
type stateReader interface {
	UnreadCount(ctx context.Context, userID string) (int, error)
}

// pushSender is the consumer-side push contract; apns.PushSender (the real
// Client or the NoOpSender) satisfies it structurally.
type pushSender interface {
	Send(ctx context.Context, tokens []string, payload json.RawMessage) ([]string, error)
}

// notificationMetricsRecorder is the consumer-side slice of the metrics registry
// the dispatchers record towncrier.notifications.created on, after a record is
// successfully persisted. *metrics.Registry satisfies it; nil leaves the counter
// dark (the default for the dispatch tests that don't assert on metrics).
type notificationMetricsRecorder interface {
	NotificationCreated(ctx context.Context, eventType, sources string)
}

// Enqueuer handles new-application zone fan-out: for a new application that
// matched a watch zone, it dedups, creates the notification record (which feeds
// the digest pipeline), and — for paid tiers with devices — sends an instant
// push, pruning any device tokens APNs reports invalid. The higher-level
// EnqueueForApplication runs the per-app zone fan-out the poll handler calls.
type Enqueuer struct {
	notifications notificationWriter
	zones         zoneMatcher
	profiles      profileReader
	devices       deviceReader
	state         stateReader
	push          pushSender
	newID         func() string
	now           func() time.Time
	logger        *slog.Logger
	metrics       notificationMetricsRecorder
}

// WithMetrics wires the recorder the enqueuer records
// towncrier.notifications.created on. A post-construction setter so the existing
// dispatch call sites and tests are unaffected; cmd/worker calls it once after
// building the enqueuer. Returns the enqueuer for chaining.
func (e *Enqueuer) WithMetrics(rec notificationMetricsRecorder) *Enqueuer {
	e.metrics = rec
	return e
}

// NewEnqueuer wires the enqueuer. newID mints the notification id (a GUID in
// production); now stamps the record's creation time. Both are injected so tests
// can pin them.
func NewEnqueuer(
	notifs notificationWriter,
	zones zoneMatcher,
	profs profileReader,
	devices deviceReader,
	state stateReader,
	push pushSender,
	newID func() string,
	now func() time.Time,
	logger *slog.Logger,
) *Enqueuer {
	return &Enqueuer{
		notifications: notifs,
		zones:         zones,
		profiles:      profs,
		devices:       devices,
		state:         state,
		push:          push,
		newID:         newID,
		now:           now,
		logger:        logger,
	}
}

// EnqueueForApplication runs the new-application zone fan-out for one polled
// application: it finds every watch zone whose circle contains the application's
// coordinates (cross-partition) and enqueues a NewApplication notification for
// each zone created on or before the application's LastDifferent timestamp. A
// zone created after the application last changed is skipped — its owner only
// subscribes to changes from creation onward, so a back-dated application is not
// "new" to them. An application without coordinates fans out to nothing.
func (e *Enqueuer) EnqueueForApplication(ctx context.Context, app applications.PlanningApplication) error {
	if app.Latitude == nil || app.Longitude == nil {
		return nil
	}
	zones, err := e.zones.FindZonesContaining(ctx, *app.Latitude, *app.Longitude)
	if err != nil {
		return err
	}
	for _, zone := range zones {
		if zone.CreatedAt.After(app.LastDifferent) {
			continue
		}
		if err := e.Enqueue(ctx, app, zone); err != nil {
			return err
		}
	}
	return nil
}

// Enqueue runs the per-(zone, application) fan-out for a NewApplication event.
// It is idempotent: a re-poll that finds an existing notification for
// (user, application, authority, NewApplication) is a no-op.
func (e *Enqueuer) Enqueue(ctx context.Context, app applications.PlanningApplication, zone watchzones.WatchZone) error {
	existing, err := e.notifications.GetByUserAndApplication(ctx, zone.UserID, app.UID, app.AreaID, notifications.EventNewApplication)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	profile, err := e.profiles.Get(ctx, zone.UserID)
	if err != nil {
		// A missing profile is not a fan-out failure — the user may have been
		// deleted between zone creation and this poll. Skip them.
		e.logger.WarnContext(ctx, "enqueue: profile unavailable, skipping", "user", zone.UserID, "error", err)
		return nil
	}
	if profile == nil {
		return nil
	}

	zoneID := zone.ID
	n := newRecord(recordInput{
		id:          e.newID(),
		userID:      zone.UserID,
		app:         app,
		watchZoneID: &zoneID,
		eventType:   notifications.EventNewApplication,
		sources:     sourceZone,
		now:         e.now().UTC(),
	})

	// Instant push is a paid-tier entitlement. Free-tier users — including a paid
	// tier whose subscription has lapsed (EffectiveTier) — still get the
	// notification record (picked up by the weekly digest) but no push.
	if profile.EffectiveTier(e.now()).IsPaid() && profile.Preferences.PushEnabled {
		n.PushSent = sendInstantPush(ctx, instantPushDeps{
			devices: e.devices,
			state:   e.state,
			push:    e.push,
			logger:  e.logger,
		}, zone.UserID, n)
	}

	if err := e.notifications.Create(ctx, n); err != nil {
		return err
	}
	if e.metrics != nil {
		e.metrics.NotificationCreated(ctx, string(n.EventType), n.Sources)
	}
	return nil
}
