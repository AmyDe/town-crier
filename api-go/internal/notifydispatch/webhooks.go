package notifydispatch

import (
	"context"
	"log/slog"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// webhookQueue stores a created notification for delivery to its user's
// webhook. zoneName is "" for a record with no watch zone. *webhooks.Outbox
// satisfies it.
type webhookQueue interface {
	Enqueue(ctx context.Context, n notifications.DigestNotification, app applications.PlanningApplication, zoneName string) error
}

// queueWebhook queues n when webhooks are wired and the user is Pro at now.
// Push preferences do not apply to webhooks. A failure is only logged: the
// notification record already exists, so a returned error would not get it
// queued on a re-dispatch.
func queueWebhook(
	ctx context.Context,
	q webhookQueue,
	logger *slog.Logger,
	profile *profiles.UserProfile,
	now time.Time,
	n notifications.DigestNotification,
	app applications.PlanningApplication,
	zoneName string,
) {
	if q == nil || !profile.EffectiveTier(now).IsPaidPro() {
		return
	}
	if err := q.Enqueue(ctx, n, app, zoneName); err != nil {
		logger.WarnContext(ctx, "enqueue: webhook outbox failed", "user", n.UserID, "notification", n.ID, "error", err)
	}
}
