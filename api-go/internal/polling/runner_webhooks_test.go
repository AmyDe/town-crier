package polling

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/webhooks"
)

type fakeWebhookDrainer struct {
	w         *runWorld
	calls     int
	remaining time.Duration
	err       error
}

func (f *fakeWebhookDrainer) Drain(ctx context.Context) webhooks.DrainResult {
	f.calls++
	f.err = ctx.Err()
	if deadline, ok := ctx.Deadline(); ok {
		f.remaining = time.Until(deadline)
	}
	f.w.events = append(f.w.events, "drain")
	return webhooks.DrainResult{}
}

func rigWithDrainer(t *testing.T) (*runnerRig, *fakeWebhookDrainer) {
	t.Helper()
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.sw.onChecks = 0
	drainer := &fakeWebhookDrainer{w: r.world}
	r.runner.d.Webhooks = drainer
	return r, drainer
}

func TestRunner_DrainsWebhooksAfterFlush(t *testing.T) {
	t.Parallel()
	r, drainer := rigWithDrainer(t)

	if _, err := r.runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []string{"reset", "dispatch", "flush", "drain"}
	if !slices.Equal(r.world.events, want) || drainer.calls != 1 {
		t.Fatalf("events = %v, want %v", r.world.events, want)
	}
}

func TestRunner_WebhookDrainHasItsOwnTimeout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		configured time.Duration
		want       time.Duration
	}{
		{"default of two minutes", 0, 2 * time.Minute},
		{"configured", 20 * time.Minute, 20 * time.Minute},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, drainer := rigWithDrainer(t)
			r.runner.cfg.EndOfRunTimeout = time.Hour
			r.runner.cfg.WebhookDrainTimeout = tc.configured

			if _, err := r.runner.Run(context.Background()); err != nil {
				t.Fatal(err)
			}

			if drainer.remaining > tc.want || drainer.remaining < tc.want-10*time.Second {
				t.Fatalf("drain time limit = %v, want about %v", drainer.remaining, tc.want)
			}
		})
	}
}

type expiringFlusher struct{ *fakeRunFlusher }

func (f expiringFlusher) Flush(ctx context.Context) error {
	<-ctx.Done()
	return f.fakeRunFlusher.Flush(ctx)
}

func TestRunner_WebhookDrainSurvivesAnExpiredFlushBudget(t *testing.T) {
	t.Parallel()
	r, drainer := rigWithDrainer(t)
	r.runner.cfg.EndOfRunTimeout = time.Millisecond
	r.runner.d.Flusher = expiringFlusher{r.flusher}

	if _, err := r.runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	if drainer.calls != 1 || drainer.err != nil {
		t.Fatalf("drain calls = %d context error = %v, want a live context", drainer.calls, drainer.err)
	}
}

func TestRunner_CancelledRunSkipsWebhookDrain(t *testing.T) {
	t.Parallel()
	r, drainer := rigWithDrainer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.sw.onChecks = -1
	r.runner.d.Switch = cancelOnCheck{cancel: cancel, next: r.sw}

	res, err := r.runner.Run(ctx)

	if err != nil || res.Stop != StopCancelled {
		t.Fatalf("stop = %s err = %v", res.Stop, err)
	}
	if r.flusher.flush != 1 || drainer.calls != 0 {
		t.Fatalf("flush = %d drain = %d, want a flush and no drain", r.flusher.flush, drainer.calls)
	}
}

type cancellingFlusher struct {
	*fakeRunFlusher
	cancel context.CancelFunc
}

func (f cancellingFlusher) Flush(ctx context.Context) error {
	f.cancel()
	return f.fakeRunFlusher.Flush(ctx)
}

func TestRunner_CancelDuringFlushSkipsWebhookDrain(t *testing.T) {
	t.Parallel()
	r, drainer := rigWithDrainer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.runner.d.Flusher = cancellingFlusher{r.flusher, cancel}

	if _, err := r.runner.Run(ctx); err != nil {
		t.Fatal(err)
	}

	if drainer.calls != 0 {
		t.Fatalf("drain = %d, want 0 after a stop signal", drainer.calls)
	}
}

type stoppedDrainer struct {
	cancel context.CancelFunc
	err    error
}

func (f *stoppedDrainer) Drain(ctx context.Context) webhooks.DrainResult {
	f.cancel()
	select {
	case <-ctx.Done():
		f.err = ctx.Err()
	case <-time.After(time.Second):
	}
	return webhooks.DrainResult{}
}

func TestRunner_CancelDuringWebhookDrainCancelsItAndReleasesLease(t *testing.T) {
	t.Parallel()
	r, _ := rigWithDrainer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	drainer := &stoppedDrainer{cancel: cancel}
	r.runner.d.Webhooks = drainer

	if _, err := r.runner.Run(ctx); err != nil {
		t.Fatal(err)
	}

	if !errors.Is(drainer.err, context.Canceled) {
		t.Fatalf("drain context error = %v, want %v", drainer.err, context.Canceled)
	}
	if r.lease.released != 1 || r.lease.releaseErr != nil {
		t.Fatalf("released = %d releaseCtxErr = %v, want one release on a live context", r.lease.released, r.lease.releaseErr)
	}
}

func TestRunner_NilWebhooksIsNoOp(t *testing.T) {
	t.Parallel()
	r := newRunnerRig(t, londonAt(6, 10, 20, 0), nil)
	r.sw.onChecks = 0

	if _, err := r.runner.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []string{"reset", "dispatch", "flush"}
	if !slices.Equal(r.world.events, want) {
		t.Fatalf("events = %v, want %v", r.world.events, want)
	}
}
