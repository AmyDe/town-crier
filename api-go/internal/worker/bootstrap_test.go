package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/servicebus"
)

// fakeTriggerQueue is a hand-written double for the bootstrapper's consumer-side
// triggerQueue interface. It records publish calls and can be primed with a
// queue depth and/or errors.
type fakeTriggerQueue struct {
	depth        servicebus.QueueDepth
	depthErr     error
	publishErr   error
	publishCalls int
	publishedAt  time.Time
	publishedBig []byte
}

func (f *fakeTriggerQueue) QueueDepth(context.Context) (servicebus.QueueDepth, error) {
	if f.depthErr != nil {
		return servicebus.QueueDepth{}, f.depthErr
	}
	return f.depth, nil
}

func (f *fakeTriggerQueue) PublishAt(_ context.Context, at time.Time, body []byte) error {
	f.publishCalls++
	f.publishedAt = at
	f.publishedBig = body
	return f.publishErr
}

func newTestBootstrapper(t *testing.T, q *fakeTriggerQueue) *Bootstrapper {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	return NewBootstrapper(q, logger, func() time.Time { return now })
}

func TestBootstrapper_PublishesSeedWhenQueueEmpty(t *testing.T) {
	t.Parallel()
	q := &fakeTriggerQueue{depth: servicebus.QueueDepth{}}
	b := newTestBootstrapper(t, q)

	res, err := b.TryBootstrap(context.Background())
	if err != nil {
		t.Fatalf("TryBootstrap: %v", err)
	}
	if !res.Published {
		t.Error("Published: got false, want true (empty queue should seed)")
	}
	if res.ProbeFailed {
		t.Error("ProbeFailed: got true, want false")
	}
	if q.publishCalls != 1 {
		t.Fatalf("publish calls: got %d, want exactly 1", q.publishCalls)
	}
	// The seed is scheduled in the future (jittered natural cadence), never
	// enqueued immediately.
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	if !q.publishedAt.After(now) {
		t.Errorf("publishedAt: got %v, want strictly after %v", q.publishedAt, now)
	}
	if len(q.publishedBig) == 0 {
		t.Error("published body is empty; want a diagnostic payload")
	}
}

func TestBootstrapper_SkipsWhenQueueNotEmpty(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		depth servicebus.QueueDepth
	}{
		{"active message present", servicebus.QueueDepth{ActiveMessageCount: 1}},
		{"scheduled message present", servicebus.QueueDepth{ScheduledMessageCount: 1}},
		{"both present", servicebus.QueueDepth{ActiveMessageCount: 2, ScheduledMessageCount: 3}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q := &fakeTriggerQueue{depth: tc.depth}
			b := newTestBootstrapper(t, q)

			res, err := b.TryBootstrap(context.Background())
			if err != nil {
				t.Fatalf("TryBootstrap: %v", err)
			}
			if res.Published {
				t.Error("Published: got true, want false (chain is alive)")
			}
			if q.publishCalls != 0 {
				t.Errorf("publish calls: got %d, want 0 (must not reseed a live chain)", q.publishCalls)
			}
		})
	}
}

func TestBootstrapper_ProbeFailureIsAbsorbed(t *testing.T) {
	t.Parallel()
	q := &fakeTriggerQueue{depthErr: errors.New("network down")}
	b := newTestBootstrapper(t, q)

	res, err := b.TryBootstrap(context.Background())
	if err != nil {
		t.Fatalf("TryBootstrap should absorb probe failures, got err=%v", err)
	}
	if res.Published {
		t.Error("Published: got true, want false (probe failed)")
	}
	if !res.ProbeFailed {
		t.Error("ProbeFailed: got false, want true")
	}
	if q.publishCalls != 0 {
		t.Errorf("publish calls: got %d, want 0 (no publish after probe failure)", q.publishCalls)
	}
}

func TestBootstrapper_PublishFailureIsAbsorbed(t *testing.T) {
	t.Parallel()
	q := &fakeTriggerQueue{depth: servicebus.QueueDepth{}, publishErr: errors.New("send rejected")}
	b := newTestBootstrapper(t, q)

	res, err := b.TryBootstrap(context.Background())
	if err != nil {
		t.Fatalf("TryBootstrap should absorb publish failures, got err=%v", err)
	}
	if res.Published {
		t.Error("Published: got true, want false (publish failed)")
	}
	if !res.ProbeFailed {
		t.Error("ProbeFailed: got false, want true (a failed publish is recorded as a probe failure for telemetry parity)")
	}
}

func TestNextSeedDelay_JitteredWithinBounds(t *testing.T) {
	t.Parallel()
	// The natural cadence is 5 minutes; the seed is jittered up to the bound on
	// either side, never producing a non-positive delay.
	for range 200 {
		d := nextSeedDelay()
		if d < naturalCadence-jitterBound || d > naturalCadence+jitterBound {
			t.Fatalf("nextSeedDelay %v out of [%v, %v]", d, naturalCadence-jitterBound, naturalCadence+jitterBound)
		}
		if d <= 0 {
			t.Fatalf("nextSeedDelay must be positive, got %v", d)
		}
	}
}
