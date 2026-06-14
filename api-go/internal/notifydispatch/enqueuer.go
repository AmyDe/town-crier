// Package notifydispatch is the poll-path notification fan-out: the per-app
// orchestration the Go poll-sb handler runs after upserting a changed planning
// application. It ports the two .NET command handlers the .NET
// PollPlanItCommandHandler invokes per app — DispatchNotificationCommandHandler
// (new-application zone fan-out) as the Enqueuer, and
// DispatchDecisionEventCommandHandler (non-decision -> decision transition) as
// the DecisionDispatcher.
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
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
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

// stateReader supplies the unread-badge inputs: the read watermark and the
// strictly-after-watermark count. *notificationstate.CosmosStore satisfies it.
type stateReader interface {
	Get(ctx context.Context, userID string) (*notificationstate.State, error)
	UnreadCount(ctx context.Context, userID string, lastReadAt time.Time) (int, error)
}

// pushSender is the consumer-side push contract; apns.PushSender (the real
// Client or the NoOpSender) satisfies it structurally.
type pushSender interface {
	Send(ctx context.Context, tokens []string, payload json.RawMessage) ([]string, error)
}

// Enqueuer ports .NET DispatchNotificationCommandHandler: for a new application
// that matched a watch zone, it dedups, creates the notification record (which
// feeds the digest pipeline), and — for paid tiers with devices — sends an
// instant push, pruning any device tokens APNs reports invalid.
type Enqueuer struct {
	notifications notificationWriter
	profiles      profileReader
	devices       deviceReader
	state         stateReader
	push          pushSender
	newID         func() string
	now           func() time.Time
	logger        *slog.Logger
}

// NewEnqueuer wires the enqueuer. newID mints the notification id (a GUID in
// production); now stamps the record's creation time. Both are injected so tests
// can pin them.
func NewEnqueuer(
	notifs notificationWriter,
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
		profiles:      profs,
		devices:       devices,
		state:         state,
		push:          push,
		newID:         newID,
		now:           now,
		logger:        logger,
	}
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
		// deleted between zone creation and this poll. Skip them, matching .NET's
		// null-profile early return.
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

	// Instant push is a paid-tier entitlement. Free-tier users still get the
	// notification record (picked up by the weekly digest) but no push.
	if profile.Tier.IsPaid() && profile.Preferences.PushEnabled {
		n.PushSent = sendInstantPush(ctx, instantPushDeps{
			devices: e.devices,
			state:   e.state,
			push:    e.push,
			logger:  e.logger,
		}, zone.UserID, n)
	}

	return e.notifications.Create(ctx, n)
}
