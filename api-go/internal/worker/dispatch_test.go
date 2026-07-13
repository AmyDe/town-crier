package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/AmyDe/town-crier/api-go/internal/polling"
	"github.com/AmyDe/town-crier/api-go/internal/servicebus"
)

// recordSingleSpan swaps in an in-memory SDK TracerProvider for the duration
// of run, restoring the previous global provider on cleanup, and returns the
// single span recorded. Deliberately not t.Parallel(): mutating the global
// TracerProvider is safe only while no sibling test's body is concurrently
// executing (the existing middleware span tests use the same non-parallel
// convention).
func recordSingleSpan(t *testing.T, run func()) sdktrace.ReadOnlySpan {
	t.Helper()

	prev := otel.GetTracerProvider()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})

	run()

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 recorded span, got %d", len(spans))
	}
	return spans[0]
}

// recordBootstrapSpan is recordSingleSpan under the name the poll-bootstrap
// span tests were written against; kept as a thin alias so those tests read
// the same as before.
func recordBootstrapSpan(t *testing.T, run func()) sdktrace.ReadOnlySpan {
	t.Helper()
	return recordSingleSpan(t, run)
}

// attrBool returns the bool value of the named attribute on the span and
// whether it was present.
func attrBool(span sdktrace.ReadOnlySpan, key string) (bool, bool) {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsBool(), true
		}
	}
	return false, false
}

// attrInt returns the int64 value of the named attribute on the span and
// whether it was present.
func attrInt(span sdktrace.ReadOnlySpan, key string) (int64, bool) {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsInt64(), true
		}
	}
	return 0, false
}

// attrFloat64 returns the float64 value of the named attribute on the span
// and whether it was present.
func attrFloat64(span sdktrace.ReadOnlySpan, key string) (float64, bool) {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsFloat64(), true
		}
	}
	return 0, false
}

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

// fakeSweep is a hand-written double for the SweepRunner the dispatcher invokes.
// It records the call and can be primed with a downgraded count or error.
type fakeSweep struct {
	calls      int
	downgraded int
	err        error
}

func (f *fakeSweep) Run(context.Context) (int, error) {
	f.calls++
	return f.downgraded, f.err
}

// fakePurge is a hand-written double for the PurgeRunner the dispatcher invokes.
// It records the call and can be primed with purge counts or an error.
type fakePurge struct {
	calls         int
	notifsPurged  int
	devicesPurged int
	err           error
}

func (f *fakePurge) Run(context.Context) (int, int, error) {
	f.calls++
	return f.notifsPurged, f.devicesPurged, f.err
}

// fakeDevSeed is a hand-written double for the DevSeedRunner the dispatcher
// invokes. It records the call and can be primed with an ingested count or an
// error.
type fakeDevSeed struct {
	calls    int
	ingested int
	err      error
}

func (f *fakeDevSeed) Run(context.Context) (int, error) {
	f.calls++
	return f.ingested, f.err
}

func TestRun_UnsetModeFailsFast(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "", nil, nil, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "digest", nil, d, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "hourly-digest", nil, d, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "digest", nil, nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when digest handler is unconfigured", code)
	}
}

func TestRun_DigestCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	d := &fakeDigester{weeklyErr: errors.New("cosmos down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "digest", nil, d, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "poll-sb", nil, nil, nil, o, nil, nil, nil, logger)

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

	code := Run(context.Background(), "poll-sb", nil, nil, nil, nil, nil, nil, nil, logger)

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
			code := Run(context.Background(), "poll-sb", nil, nil, nil, o, nil, nil, nil, logger)
			if code != tc.wantExit {
				t.Errorf("exit code: got %d, want %d", code, tc.wantExit)
			}
		})
	}
}

// TestRun_PollSBStampsOldestHWMAttributesOnSpan pins tc-3jx8d: the oldest-HWM
// staleness the polling handler already computes must land on the "Polling
// Cycle (SB)" span so it's queryable in App Insights (the OTel metrics
// registry alone never reaches it).
func TestRun_PollSBStampsOldestHWMAttributesOnSpan(t *testing.T) {
	age := 345600.0 // 4 days, seconds
	o := &fakePollOrchestrator{result: PollRunResult{
		MessageReceived:      true,
		OldestHWMAgeSeconds:  &age,
		OldestHWMNeverPolled: false,
	}}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	span := recordSingleSpan(t, func() {
		Run(context.Background(), "poll-sb", nil, nil, nil, o, nil, nil, nil, logger)
	})

	if span.Name() != "Polling Cycle (SB)" {
		t.Fatalf("span name: got %q, want %q", span.Name(), "Polling Cycle (SB)")
	}
	got, ok := attrFloat64(span, "polling.oldest_hwm_age_seconds")
	if !ok {
		t.Fatalf("missing polling.oldest_hwm_age_seconds; attrs=%v", span.Attributes())
	}
	if got != age {
		t.Errorf("polling.oldest_hwm_age_seconds: got %v, want %v", got, age)
	}
	neverPolled, ok := attrBool(span, "polling.oldest_hwm_never_polled")
	if !ok {
		t.Fatalf("missing polling.oldest_hwm_never_polled; attrs=%v", span.Attributes())
	}
	if neverPolled {
		t.Errorf("polling.oldest_hwm_never_polled: got true, want false")
	}
}

// TestRun_PollSBOmitsOldestHWMAttributesWhenAbsent covers the empty
// candidate-set case: the handler records nothing, so the span must not carry
// a misleading zero value for either attribute.
func TestRun_PollSBOmitsOldestHWMAttributesWhenAbsent(t *testing.T) {
	o := &fakePollOrchestrator{result: PollRunResult{MessageReceived: true}}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	span := recordSingleSpan(t, func() {
		Run(context.Background(), "poll-sb", nil, nil, nil, o, nil, nil, nil, logger)
	})

	if _, ok := attrFloat64(span, "polling.oldest_hwm_age_seconds"); ok {
		t.Error("polling.oldest_hwm_age_seconds: present, want absent when no candidate was recorded")
	}
	if _, ok := attrBool(span, "polling.oldest_hwm_never_polled"); ok {
		t.Error("polling.oldest_hwm_never_polled: present, want absent when no candidate was recorded")
	}
}

func TestRun_PollSBExitsOneOnOrchestratorError(t *testing.T) {
	t.Parallel()
	o := &fakePollOrchestrator{err: errors.New("orchestrator blew up")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "poll-sb", nil, nil, nil, o, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on orchestrator error", code)
	}
}

func TestRun_DormantCleanupRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	d := &fakeDormant{deleted: 3}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "dormant-cleanup", nil, nil, d, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "dormant-cleanup", nil, nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when dormant handler is unconfigured", code)
	}
}

func TestRun_DormantCleanupCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	d := &fakeDormant{err: errors.New("cosmos down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "dormant-cleanup", nil, nil, d, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on dormant cleanup error", code)
	}
}

func TestRun_SubscriptionSweepRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	s := &fakeSweep{downgraded: 4}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "subscription-sweep", nil, nil, nil, nil, s, nil, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 (successful subscription sweep)", code)
	}
	if s.calls != 1 {
		t.Errorf("sweep Run calls: got %d, want 1", s.calls)
	}
}

func TestRun_SubscriptionSweepWithoutHandlerExitsOne(t *testing.T) {
	t.Parallel()
	// A job missing Cosmos config leaves the sweep runner nil; the mode must
	// refuse to run rather than nil-panic.
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "subscription-sweep", nil, nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when sweep handler is unconfigured", code)
	}
}

func TestRun_SubscriptionSweepCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	s := &fakeSweep{err: errors.New("cosmos down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "subscription-sweep", nil, nil, nil, nil, s, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on subscription sweep error", code)
	}
}

func TestRun_PgPurgeRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	p := &fakePurge{notifsPurged: 12, devicesPurged: 3}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "pg-purge", nil, nil, nil, nil, nil, p, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 (successful pg-purge)", code)
	}
	if p.calls != 1 {
		t.Errorf("purge Run calls: got %d, want 1", p.calls)
	}
}

func TestRun_PgPurgeWithNilRunnerExitsZero(t *testing.T) {
	t.Parallel()
	// When no purge runner is configured the purger is nil; pg-purge must exit 0
	// (not 1) — an unconfigured pg-purge is a deliberate safe no-op.
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "pg-purge", nil, nil, nil, nil, nil, nil, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 when purger is nil (Cosmos TTL active)", code)
	}
	if !strings.Contains(buf.String(), "Cosmos TTL") {
		t.Errorf("log should mention Cosmos TTL, got: %s", buf.String())
	}
}

func TestRun_PgPurgeCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	p := &fakePurge{err: errors.New("postgres down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "pg-purge", nil, nil, nil, nil, nil, p, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on pg-purge error", code)
	}
}

func TestRun_UnknownModeExitsOne(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "banana", nil, nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 for unknown mode", code)
	}
}

func TestRun_PollBootstrapSeedsAndExitsZero(t *testing.T) {
	t.Parallel()
	q := &fakeTriggerQueue{depth: servicebus.QueueDepth{}}
	b := newTestBootstrapper(t, q)
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "poll-bootstrap", b, nil, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "poll-bootstrap", nil, nil, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "poll-bootstrap", b, nil, nil, nil, nil, nil, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 (absorbed probe failure is not a job failure)", code)
	}
}

// TestRunPollBootstrap_TagsReconciliationAttributes proves the "Polling
// Bootstrap" span surfaces the GH#938 PR1/PR2 BootstrapResult fields as
// attributes, so App Insights can alert on a fork without a human happening to
// look: a forked queue (2 scheduled + 1 active) reconciled down to one trigger
// tags polling.safety_net.reconciled/scheduled_cancelled/active_discarded, and
// a non-empty DLQ drain tags dead_lettered — additive telemetry only, no
// dispatch behaviour change.
func TestRunPollBootstrap_TagsReconciliationAttributes(t *testing.T) {
	q := &fakeTriggerQueue{
		depth: servicebus.QueueDepth{ActiveMessageCount: 1, ScheduledMessageCount: 2},
		peeked: []servicebus.PeekedMessage{
			{SequenceNumber: 10, State: servicebus.MessageStateActive},
			{SequenceNumber: 20, State: servicebus.MessageStateScheduled},
			{SequenceNumber: 21, State: servicebus.MessageStateScheduled},
		},
		receiveResult: true,
		dlqDrained:    3,
	}
	b := newTestBootstrapper(t, q)
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	span := recordBootstrapSpan(t, func() {
		code := Run(context.Background(), "poll-bootstrap", b, nil, nil, nil, nil, nil, nil, logger)
		if code != 0 {
			t.Errorf("exit code: got %d, want 0", code)
		}
	})

	if got, ok := attrBool(span, "polling.safety_net.reconciled"); !ok || !got {
		t.Errorf("polling.safety_net.reconciled: got %v (ok=%v), want true", got, ok)
	}
	if got, ok := attrInt(span, "polling.safety_net.scheduled_cancelled"); !ok || got != 1 {
		t.Errorf("polling.safety_net.scheduled_cancelled: got %d (ok=%v), want 1", got, ok)
	}
	if got, ok := attrInt(span, "polling.safety_net.active_discarded"); !ok || got != 1 {
		t.Errorf("polling.safety_net.active_discarded: got %d (ok=%v), want 1", got, ok)
	}
	if got, ok := attrInt(span, "polling.safety_net.dead_lettered"); !ok || got != 3 {
		t.Errorf("polling.safety_net.dead_lettered: got %d (ok=%v), want 3", got, ok)
	}
	if got, ok := attrBool(span, "polling.safety_net.lease_unavailable"); !ok || got {
		t.Errorf("polling.safety_net.lease_unavailable: got %v (ok=%v), want false", got, ok)
	}
}

// TestRunPollBootstrap_TagsLeaseUnavailableAttribute proves the PR1
// LeaseUnavailable field (never previously surfaced) is now tagged on the
// span: when a peer holds the polling lease, the span must report
// lease_unavailable=true and every reconciliation count at its zero value
// (nothing was probed or touched).
func TestRunPollBootstrap_TagsLeaseUnavailableAttribute(t *testing.T) {
	q := &fakeTriggerQueue{depth: servicebus.QueueDepth{}}
	lease := &fakeLeaseAccess{acquireResult: polling.LeaseAcquireResult{Held: true}}
	b := newTestBootstrapperWithLease(t, q, lease)
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	span := recordBootstrapSpan(t, func() {
		code := Run(context.Background(), "poll-bootstrap", b, nil, nil, nil, nil, nil, nil, logger)
		if code != 0 {
			t.Errorf("exit code: got %d, want 0", code)
		}
	})

	if got, ok := attrBool(span, "polling.safety_net.lease_unavailable"); !ok || !got {
		t.Errorf("polling.safety_net.lease_unavailable: got %v (ok=%v), want true", got, ok)
	}
	if got, ok := attrBool(span, "polling.safety_net.reconciled"); !ok || got {
		t.Errorf("polling.safety_net.reconciled: got %v (ok=%v), want false", got, ok)
	}
	if got, ok := attrInt(span, "polling.safety_net.dead_lettered"); !ok || got != 0 {
		t.Errorf("polling.safety_net.dead_lettered: got %d (ok=%v), want 0 (lease held; never probed)", got, ok)
	}
}

func TestRun_DevSeedRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	ds := &fakeDevSeed{ingested: 3}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "dev-seed", nil, nil, nil, nil, nil, nil, ds, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 (successful dev-seed cycle)", code)
	}
	if ds.calls != 1 {
		t.Errorf("dev-seed Run calls: got %d, want 1", ds.calls)
	}
}

func TestRun_DevSeedWithoutRunnerExitsOne(t *testing.T) {
	t.Parallel()
	// A job missing its dedicated prod-read config (DEV_SEED_PROD_AZURE_CLIENT_ID
	// / DEV_SEED_PROD_POSTGRES_USER) leaves the dev-seed runner nil; the mode must
	// refuse to run rather than nil-panic.
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "dev-seed", nil, nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when dev-seed is unconfigured", code)
	}
}

func TestRun_DevSeedCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	ds := &fakeDevSeed{err: errors.New("postgres down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "dev-seed", nil, nil, nil, nil, nil, nil, ds, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on dev-seed cycle error", code)
	}
}
