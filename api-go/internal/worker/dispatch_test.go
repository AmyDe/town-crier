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

// fakeDormant is a hand-written double for the DormantRunner the dispatcher
// invokes. It records the call and can be primed with a deleted count or error.
type fakeDormant struct {
	calls   int
	deleted int
	err     error
}

func (f *fakeDormant) Run(context.Context) (int, error) {
	f.calls++
	return f.deleted, f.err
}

func TestRun_UnsetModeFailsFast(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "", nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "digest", nil, d, nil, nil, logger)

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

	code := Run(context.Background(), "hourly-digest", nil, d, nil, nil, logger)

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

	code := Run(context.Background(), "digest", nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when digest handler is unconfigured", code)
	}
}

func TestRun_DigestCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	d := &fakeDigester{weeklyErr: errors.New("cosmos down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "digest", nil, d, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on digest cycle error", code)
	}
}

// fakePollOrchestrator is a hand-written double for the poll-sb orchestrator the
// dispatcher invokes. It records the call and can be primed with a run result or
// error.
type fakePollOrchestrator struct {
	calls  int
	result PollRunResult
	err    error
}

func (f *fakePollOrchestrator) RunOnce(context.Context) (PollRunResult, error) {
	f.calls++
	return f.result, f.err
}

func TestRun_PollSBRunsOrchestratorAndExitsZeroOnSuccess(t *testing.T) {
	t.Parallel()
	o := &fakePollOrchestrator{result: PollRunResult{
		MessageReceived:   true,
		PublishedNext:     true,
		ApplicationCount:  5,
		AuthoritiesPolled: 2,
	}}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "poll-sb", nil, nil, nil, o, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 for a successful poll cycle", code)
	}
	if o.calls != 1 {
		t.Errorf("orchestrator calls: got %d, want 1", o.calls)
	}
}

func TestRun_PollSBWithoutOrchestratorExitsOne(t *testing.T) {
	t.Parallel()
	// A job missing Service Bus / Cosmos config leaves the orchestrator nil; the
	// mode must refuse to run rather than nil-panic.
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "poll-sb", nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when poll-sb is unconfigured", code)
	}
}

func TestRun_PollSBExitsOneOnlyWhenNoAppsAndAuthorityErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		result   PollRunResult
		wantExit int
	}{
		{
			name:     "no apps and authority errors -> exit 1",
			result:   PollRunResult{MessageReceived: true, ApplicationCount: 0, AuthorityErrors: 2},
			wantExit: 1,
		},
		{
			name:     "no apps but no authority errors -> exit 0 (quiet cycle)",
			result:   PollRunResult{MessageReceived: true, ApplicationCount: 0, AuthorityErrors: 0},
			wantExit: 0,
		},
		{
			name:     "apps ingested despite some authority errors -> exit 0",
			result:   PollRunResult{MessageReceived: true, ApplicationCount: 10, AuthorityErrors: 1},
			wantExit: 0,
		},
		{
			name:     "lease unavailable -> exit 0 (peer is polling)",
			result:   PollRunResult{LeaseUnavailable: true},
			wantExit: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := &fakePollOrchestrator{result: tc.result}
			logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
			code := Run(context.Background(), "poll-sb", nil, nil, nil, o, logger)
			if code != tc.wantExit {
				t.Errorf("exit code: got %d, want %d", code, tc.wantExit)
			}
		})
	}
}

func TestRun_PollSBExitsOneOnOrchestratorError(t *testing.T) {
	t.Parallel()
	o := &fakePollOrchestrator{err: errors.New("orchestrator blew up")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "poll-sb", nil, nil, nil, o, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on orchestrator error", code)
	}
}

func TestRun_DormantCleanupRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	d := &fakeDormant{deleted: 3}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "dormant-cleanup", nil, nil, d, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 (successful dormant cleanup)", code)
	}
	if d.calls != 1 {
		t.Errorf("dormant Run calls: got %d, want 1", d.calls)
	}
}

func TestRun_DormantCleanupWithoutHandlerExitsOne(t *testing.T) {
	t.Parallel()
	// A job missing Cosmos config leaves the dormant runner nil; the mode must
	// refuse to run rather than nil-panic.
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "dormant-cleanup", nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when dormant handler is unconfigured", code)
	}
}

func TestRun_DormantCleanupCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	d := &fakeDormant{err: errors.New("cosmos down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "dormant-cleanup", nil, nil, d, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on dormant cleanup error", code)
	}
}

func TestRun_UnknownModeExitsOne(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "banana", nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 for unknown mode", code)
	}
}

func TestRun_PollBootstrapSeedsAndExitsZero(t *testing.T) {
	t.Parallel()
	q := &fakeTriggerQueue{depth: servicebus.QueueDepth{}}
	b := newTestBootstrapper(t, q)
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "poll-bootstrap", b, nil, nil, nil, logger)

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

	code := Run(context.Background(), "poll-bootstrap", nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "poll-bootstrap", b, nil, nil, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 (absorbed probe failure is not a job failure)", code)
	}
}
