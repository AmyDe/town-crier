package polling

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// gateNow is the fixed clock the recency-gate tests evaluate the window
// against.
func gateNow() time.Time { return time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC) }

// recencyGateWindow is the 30-day default (POLLING_LANE_E_NOTIFY_RECENCY_DAYS).
const recencyGateWindow = 30 * 24 * time.Hour

func ptrTime(t time.Time) *time.Time { return &t }

// TestRecencyGatedEnqueuer_GatesOnStartDate pins ADR 0047's new-application
// gate: EnqueueForApplication calls through only when StartDate is present AND
// no older than now-window; a nil or too-old StartDate returns nil without
// touching the inner enqueuer (fail closed).
func TestRecencyGatedEnqueuer_GatesOnStartDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		startDate   *time.Time
		wantThrough bool
	}{
		{"inside window (yesterday)", ptrTime(gateNow().AddDate(0, 0, -1)), true},
		{"exactly on the window edge", ptrTime(gateNow().Add(-recencyGateWindow)), true},
		{"just outside the window", ptrTime(gateNow().Add(-recencyGateWindow - time.Hour)), false},
		{"far outside (80 days ago)", ptrTime(gateNow().AddDate(0, 0, -80)), false},
		{"nil start date fails closed", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inner := &fakeEnqueuer{}
			gate := recencyGatedEnqueuer{inner: inner, window: recencyGateWindow, now: gateNow}

			app := applications.PlanningApplication{UID: "26/0001/FUL", AreaID: 300, StartDate: tc.startDate}
			if err := gate.EnqueueForApplication(context.Background(), app); err != nil {
				t.Fatalf("EnqueueForApplication: %v", err)
			}

			if got := inner.count() > 0; got != tc.wantThrough {
				t.Errorf("called through: got %v, want %v", got, tc.wantThrough)
			}
		})
	}
}

// TestRecencyGatedDispatcher_GatesOnDecidedDate is the decision counterpart:
// Dispatch calls through only when DecidedDate is present AND recent; a nil or
// too-old DecidedDate returns nil without touching the inner dispatcher.
func TestRecencyGatedDispatcher_GatesOnDecidedDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		decidedDate *time.Time
		wantThrough bool
	}{
		{"inside window (3 days ago)", ptrTime(gateNow().AddDate(0, 0, -3)), true},
		{"exactly on the window edge", ptrTime(gateNow().Add(-recencyGateWindow)), true},
		{"just outside the window", ptrTime(gateNow().Add(-recencyGateWindow - time.Hour)), false},
		{"nil decided date fails closed", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inner := &fakeDecisionDispatcher{}
			gate := recencyGatedDispatcher{inner: inner, window: recencyGateWindow, now: gateNow}

			app := applications.PlanningApplication{UID: "26/0002/FUL", AreaID: 301, DecidedDate: tc.decidedDate}
			if err := gate.Dispatch(context.Background(), app); err != nil {
				t.Fatalf("Dispatch: %v", err)
			}

			if got := inner.count() > 0; got != tc.wantThrough {
				t.Errorf("called through: got %v, want %v", got, tc.wantThrough)
			}
		})
	}
}

// TestRecencyGates_EventSpecificDates proves each decorator judges its OWN
// date: a fresh StartDate with a stale DecidedDate still notifies the
// new-application path but not the decision path, and vice versa.
func TestRecencyGates_EventSpecificDates(t *testing.T) {
	t.Parallel()

	fresh := ptrTime(gateNow().AddDate(0, 0, -2))
	stale := ptrTime(gateNow().AddDate(0, 0, -80))

	enqInner := &fakeEnqueuer{}
	dispInner := &fakeDecisionDispatcher{}
	enqGate := recencyGatedEnqueuer{inner: enqInner, window: recencyGateWindow, now: gateNow}
	dispGate := recencyGatedDispatcher{inner: dispInner, window: recencyGateWindow, now: gateNow}

	// Fresh start_date, stale decided_date.
	app := applications.PlanningApplication{UID: "x", AreaID: 1, StartDate: fresh, DecidedDate: stale}
	if err := enqGate.EnqueueForApplication(context.Background(), app); err != nil {
		t.Fatalf("EnqueueForApplication: %v", err)
	}
	if err := dispGate.Dispatch(context.Background(), app); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if enqInner.count() != 1 {
		t.Errorf("enqueuer: got %d calls, want 1 (fresh start_date)", enqInner.count())
	}
	if dispInner.count() != 0 {
		t.Errorf("dispatcher: got %d calls, want 0 (stale decided_date)", dispInner.count())
	}
}
