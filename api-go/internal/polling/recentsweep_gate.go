package polling

import (
	"context"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// ADR 0047: Lane E's event-specific recency gate. Two decorators over the
// existing consumer-side fan-out interfaces (DecisionDispatcher,
// NotificationEnqueuer — handler.go), composed ON TOP of the real
// collaborators rather than built into them. RecentSweepHandler.WithFanOut
// wraps both itself, so the wiring site cannot hand Lane E an ungated
// notifier — the same structural instinct ADR 0042 used to omit WithFanOut
// from Lane D, applied to a lane that DOES need to notify.
//
// The gate sits ABOVE Enqueue/Dispatch, so a gated-out event produces no
// notification record at all — data minimisation by construction, not a
// suppressed push on top of a stored row.
//
// The test is event-specific: a new-application fan-out is judged on
// start_date (the council's filing date), a decision on decided_date. Neither
// reads last_different: a PlanIt re-index bumps it, so a 2019 application
// re-scraped today would read as maximally recent. Both decorators fail closed
// on a nil date — no date, no evidence of recency, no notification.

// withinRecency reports whether date is present and no older than
// now().Add(-window). A nil date fails closed (false).
func withinRecency(date *time.Time, window time.Duration, now func() time.Time) bool {
	if date == nil {
		return false
	}
	return !date.Before(now().Add(-window))
}

// recencyGatedEnqueuer drops a watch-zone fan-out whose application's
// start_date is missing or older than window.
type recencyGatedEnqueuer struct {
	inner  NotificationEnqueuer
	window time.Duration
	now    func() time.Time
}

// EnqueueForApplication calls the inner enqueuer only when the application's
// start_date is present and recent; otherwise it returns nil without a call.
func (g recencyGatedEnqueuer) EnqueueForApplication(ctx context.Context, app applications.PlanningApplication) error {
	if !withinRecency(app.StartDate, g.window, g.now) {
		return nil
	}
	return g.inner.EnqueueForApplication(ctx, app)
}

// recencyGatedDispatcher drops a decision-event dispatch whose application's
// decided_date is missing or older than window.
type recencyGatedDispatcher struct {
	inner  DecisionDispatcher
	window time.Duration
	now    func() time.Time
}

// Dispatch calls the inner dispatcher only when the application's decided_date
// is present and recent; otherwise it returns nil without a call.
func (g recencyGatedDispatcher) Dispatch(ctx context.Context, app applications.PlanningApplication) error {
	if !withinRecency(app.DecidedDate, g.window, g.now) {
		return nil
	}
	return g.inner.Dispatch(ctx, app)
}
