package pgpurge_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/pgpurge"
)

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

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

var testNow = time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func TestHandler_Run_PurgesBothStoresWithCorrectCutoffs(t *testing.T) {
	t.Parallel()

	notifs := &fakePurger{rowsResult: 42}
	devices := &fakePurger{rowsResult: 7}

	h := pgpurge.New(notifs, devices, &fakeDeliveryPurger{}, 90*24*time.Hour, 180*24*time.Hour, fixedClock(testNow), discardLogger())

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

	h := pgpurge.New(notifs, devices, &fakeDeliveryPurger{}, 90*24*time.Hour, 180*24*time.Hour, fixedClock(testNow), discardLogger())

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

	h := pgpurge.New(notifs, devices, &fakeDeliveryPurger{}, 90*24*time.Hour, 180*24*time.Hour, fixedClock(testNow), discardLogger())

	_, _, err := h.Run(context.Background())

	if err == nil {
		t.Fatal("Run: expected error from devices purge, got nil")
	}
}

func TestHandler_Run_BothCallsReceiveContext(t *testing.T) {
	t.Parallel()

	notifs := &fakePurger{}
	devices := &fakePurger{}

	h := pgpurge.New(notifs, devices, &fakeDeliveryPurger{}, 90*24*time.Hour, 180*24*time.Hour, fixedClock(testNow), discardLogger())

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

type fakePollPurger struct {
	calls   int
	lastNow time.Time
	result  map[string]int64
	err     error
}

func (f *fakePollPurger) Purge(_ context.Context, now time.Time) (map[string]int64, error) {
	f.calls++
	f.lastNow = now
	return f.result, f.err
}

func TestHandler_Run_PurgesPollDataWithTheClock(t *testing.T) {
	t.Parallel()

	poll := &fakePollPurger{result: map[string]int64{"planit_call": 3}}
	h := pgpurge.New(&fakePurger{}, &fakePurger{}, &fakeDeliveryPurger{}, 90*24*time.Hour, 180*24*time.Hour, fixedClock(testNow), discardLogger()).
		WithPollRetention(poll)

	if _, _, err := h.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if poll.calls != 1 || !poll.lastNow.Equal(testNow) {
		t.Errorf("poll purge calls=%d now=%v, want 1 call at %v", poll.calls, poll.lastNow, testNow)
	}
}

func TestHandler_Run_PollPurgeErrorIsReturned(t *testing.T) {
	t.Parallel()

	poll := &fakePollPurger{err: errors.New("postgres down")}
	h := pgpurge.New(&fakePurger{rowsResult: 2}, &fakePurger{rowsResult: 1}, &fakeDeliveryPurger{}, 90*24*time.Hour, 180*24*time.Hour, fixedClock(testNow), discardLogger()).
		WithPollRetention(poll)

	n, d, err := h.Run(context.Background())
	if err == nil {
		t.Fatal("Run: want error from poll purge")
	}
	if n != 2 || d != 1 {
		t.Errorf("counts = %d, %d, want the notification and device counts kept", n, d)
	}
}

func TestHandler_Run_WithoutPollRetentionStillWorks(t *testing.T) {
	t.Parallel()

	h := pgpurge.New(&fakePurger{}, &fakePurger{}, &fakeDeliveryPurger{}, 90*24*time.Hour, 180*24*time.Hour, fixedClock(testNow), discardLogger())
	if _, _, err := h.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

type fakeDeliveryPurger struct {
	calls      int
	lastCutoff time.Time
	rowsResult int64
	err        error
}

func (f *fakeDeliveryPurger) PurgeDeliveriesOlderThan(_ context.Context, cutoff time.Time) (int64, error) {
	f.calls++
	f.lastCutoff = cutoff
	return f.rowsResult, f.err
}

func TestHandler_Run_PurgesWebhookDeliveriesOlderThanSevenDays(t *testing.T) {
	t.Parallel()

	deliveries := &fakeDeliveryPurger{rowsResult: 9}
	var logs bytes.Buffer
	h := pgpurge.New(&fakePurger{}, &fakePurger{}, deliveries, 90*24*time.Hour, 180*24*time.Hour,
		fixedClock(testNow), slog.New(slog.NewJSONHandler(&logs, nil)))

	if _, _, err := h.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := testNow.Add(-7 * 24 * time.Hour)
	if deliveries.calls != 1 || !deliveries.lastCutoff.Equal(want) {
		t.Errorf("calls = %d cutoff = %v, want 1 call with %v", deliveries.calls, deliveries.lastCutoff, want)
	}
	if !strings.Contains(logs.String(), `"webhookDeliveriesDeleted":9`) {
		t.Errorf("log = %s, want the delivery count", logs.String())
	}
}

func TestHandler_Run_WebhookPurgeErrorIsReturned(t *testing.T) {
	t.Parallel()

	down := errors.New("postgres down")
	h := pgpurge.New(&fakePurger{rowsResult: 2}, &fakePurger{rowsResult: 1}, &fakeDeliveryPurger{err: down},
		90*24*time.Hour, 180*24*time.Hour, fixedClock(testNow), discardLogger())

	n, d, err := h.Run(context.Background())

	if !errors.Is(err, down) || n != 2 || d != 1 {
		t.Errorf("Run = (%d, %d, %v), want (2, 1) and the purge error", n, d, err)
	}
}
