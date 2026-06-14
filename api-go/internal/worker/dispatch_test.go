package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/servicebus"
)

// fakeDigester is a hand-written double for the digestRunner the dispatcher
// invokes. It records which cycle ran and can be primed with an error.
type fakeDigester struct {
	weeklyCalls int
	hourlyCalls int
	weeklyErr   error
	hourlyErr   error
}

func (f *fakeDigester) RunWeekly(context.Context) error {
	f.weeklyCalls++
	return f.weeklyErr
}

func (f *fakeDigester) RunHourly(context.Context) error {
	f.hourlyCalls++
	return f.hourlyErr
}

func TestRun_UnsetModeFailsFast(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "", nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 for unset mode", code)
	}
	if !strings.Contains(buf.String(), "WORKER_MODE") {
		t.Errorf("log should mention WORKER_MODE, got: %s", buf.String())
	}
}

func TestRun_DigestModeRunsWeeklyAndExitsZero(t *testing.T) {
	t.Parallel()
	d := &fakeDigester{}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "digest", nil, d, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0", code)
	}
	if d.weeklyCalls != 1 || d.hourlyCalls != 0 {
		t.Errorf("calls: weekly=%d hourly=%d, want 1/0", d.weeklyCalls, d.hourlyCalls)
	}
}

func TestRun_HourlyDigestModeRunsHourlyAndExitsZero(t *testing.T) {
	t.Parallel()
	d := &fakeDigester{}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "hourly-digest", nil, d, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0", code)
	}
	if d.hourlyCalls != 1 || d.weeklyCalls != 0 {
		t.Errorf("calls: weekly=%d hourly=%d, want 0/1", d.weeklyCalls, d.hourlyCalls)
	}
}

func TestRun_DigestModeWithoutHandlerExitsOne(t *testing.T) {
	t.Parallel()
	// A job missing Cosmos/ACS config leaves the digester nil; the mode must
	// refuse to run rather than nil-panic.
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "digest", nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when digest handler is unconfigured", code)
	}
}

func TestRun_DigestCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	d := &fakeDigester{weeklyErr: errors.New("cosmos down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "digest", nil, d, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on digest cycle error", code)
	}
}

func TestRun_UnimplementedModesExitOne(t *testing.T) {
	t.Parallel()
	// The remaining non-digest modes are stubs until their own beads land. The
	// image is not deployed until the final cutover, so failing fast (exit 1) is
	// safe and loud.
	modes := []string{"poll-sb", "dormant-cleanup"}
	for _, mode := range modes {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, nil))

			code := Run(context.Background(), mode, nil, nil, logger)

			if code != 1 {
				t.Errorf("exit code: got %d, want 1 for unimplemented mode %q", code, mode)
			}
			out := buf.String()
			if !strings.Contains(out, "not yet implemented in Go worker") {
				t.Errorf("log should report unimplemented mode, got: %s", out)
			}
			if !strings.Contains(out, mode) {
				t.Errorf("log should name the mode %q, got: %s", mode, out)
			}
		})
	}
}

func TestRun_UnknownModeExitsOne(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "banana", nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 for unknown mode", code)
	}
}

func TestRun_PollBootstrapSeedsAndExitsZero(t *testing.T) {
	t.Parallel()
	q := &fakeTriggerQueue{depth: servicebus.QueueDepth{}}
	b := newTestBootstrapper(t, q)
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "poll-bootstrap", b, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 (successful bootstrap)", code)
	}
	if q.publishCalls != 1 {
		t.Errorf("publish calls: got %d, want 1", q.publishCalls)
	}
}

func TestRun_PollBootstrapWithoutQueueExitsOne(t *testing.T) {
	t.Parallel()
	// On a job missing Service Bus config the bootstrapper is nil; poll-bootstrap
	// must refuse to run (exit 1) rather than nil-panic.
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "poll-bootstrap", nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when Service Bus is unconfigured", code)
	}
}

func TestRun_PollBootstrapProbeFailureStillExitsZero(t *testing.T) {
	t.Parallel()
	// A probe failure is absorbed by the bootstrapper (the safety net retries on
	// the next tick), so the job itself should not fail — exit 0.
	q := &fakeTriggerQueue{depthErr: errors.New("transient")}
	b := newTestBootstrapper(t, q)
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "poll-bootstrap", b, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 (absorbed probe failure is not a job failure)", code)
	}
}
