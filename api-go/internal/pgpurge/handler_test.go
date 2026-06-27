package pgpurge_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/pgpurge"
)

// fakePurger is a hand-written double for either notifPurger or devicePurger.
// It records the cutoff time it received and can be primed with a row count or
// an error.
type fakePurger struct {
	calls      int
	lastCutoff time.Time
	rowsResult int64
	err        error
}

func (f *fakePurger) PurgeOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	f.calls++
	f.lastCutoff = cutoff
	return f.rowsResult, f.err
}

// fixedClock returns a time.Time that never changes, for deterministic tests.
func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

var testNow = time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func TestHandler_Run_PurgesBothStoresWithCorrectCutoffs(t *testing.T) {
	t.Parallel()

	notifs := &fakePurger{rowsResult: 42}
	devices := &fakePurger{rowsResult: 7}

	h := pgpurge.New(notifs, devices, 90*24*time.Hour, 180*24*time.Hour, fixedClock(testNow), discardLogger())

	notifsPurged, devicesPurged, err := h.Run(context.Background())

	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if notifsPurged != 42 {
		t.Errorf("notifsPurged = %d, want 42", notifsPurged)
	}
	if devicesPurged != 7 {
		t.Errorf("devicesPurged = %d, want 7", devicesPurged)
	}

	wantNotifCutoff := testNow.Add(-90 * 24 * time.Hour)
	if !notifs.lastCutoff.Equal(wantNotifCutoff) {
		t.Errorf("notifs cutoff = %v, want %v", notifs.lastCutoff, wantNotifCutoff)
	}

	wantDeviceCutoff := testNow.Add(-180 * 24 * time.Hour)
	if !devices.lastCutoff.Equal(wantDeviceCutoff) {
		t.Errorf("devices cutoff = %v, want %v", devices.lastCutoff, wantDeviceCutoff)
	}
}

func TestHandler_Run_ErrorOnNotifsPurgeReturnsError(t *testing.T) {
	t.Parallel()

	notifs := &fakePurger{err: errors.New("postgres down")}
	devices := &fakePurger{rowsResult: 5}

	h := pgpurge.New(notifs, devices, 90*24*time.Hour, 180*24*time.Hour, fixedClock(testNow), discardLogger())

	_, _, err := h.Run(context.Background())

	if err == nil {
		t.Fatal("Run: expected error from notifs purge, got nil")
	}
	if devices.calls != 0 {
		t.Errorf("device purger called %d times after notifs error, want 0", devices.calls)
	}
}

func TestHandler_Run_ErrorOnDevicesPurgeReturnsError(t *testing.T) {
	t.Parallel()

	notifs := &fakePurger{rowsResult: 10}
	devices := &fakePurger{err: errors.New("constraint violation")}

	h := pgpurge.New(notifs, devices, 90*24*time.Hour, 180*24*time.Hour, fixedClock(testNow), discardLogger())

	_, _, err := h.Run(context.Background())

	if err == nil {
		t.Fatal("Run: expected error from devices purge, got nil")
	}
}

func TestHandler_Run_BothCallsReceiveContext(t *testing.T) {
	t.Parallel()

	notifs := &fakePurger{}
	devices := &fakePurger{}

	h := pgpurge.New(notifs, devices, 90*24*time.Hour, 180*24*time.Hour, fixedClock(testNow), discardLogger())

	if _, _, err := h.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if notifs.calls != 1 {
		t.Errorf("notifs PurgeOlderThan calls = %d, want 1", notifs.calls)
	}
	if devices.calls != 1 {
		t.Errorf("devices PurgeOlderThan calls = %d, want 1", devices.calls)
	}
}
