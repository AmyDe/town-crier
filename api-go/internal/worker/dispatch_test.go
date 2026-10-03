package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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

// fakeAppStoreReconcile is a hand-written double for the AppStoreReconcileRunner
// the dispatcher invokes. It records the call and can be primed with
// scanned/gap/applied counts or an error.
type fakeAppStoreReconcile struct {
	calls   int
	scanned int
	gaps    int
	applied int
	err     error
}

func (f *fakeAppStoreReconcile) Run(context.Context) (int, int, int, error) {
	f.calls++
	return f.scanned, f.gaps, f.applied, f.err
}

func TestRun_UnsetModeFailsFast(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "", nil, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "digest", d, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "hourly-digest", d, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "digest", nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when digest handler is unconfigured", code)
	}
}

func TestRun_DigestCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	d := &fakeDigester{weeklyErr: errors.New("cosmos down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "digest", d, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on digest cycle error", code)
	}
}

func TestRun_DormantCleanupRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	d := &fakeDormant{deleted: 3}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "dormant-cleanup", nil, d, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "dormant-cleanup", nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when dormant handler is unconfigured", code)
	}
}

func TestRun_DormantCleanupCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	d := &fakeDormant{err: errors.New("cosmos down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "dormant-cleanup", nil, d, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on dormant cleanup error", code)
	}
}

func TestRun_SubscriptionSweepRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	s := &fakeSweep{downgraded: 4}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "subscription-sweep", nil, nil, nil, s, nil, nil, logger)

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

	code := Run(context.Background(), "subscription-sweep", nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when sweep handler is unconfigured", code)
	}
}

func TestRun_SubscriptionSweepCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	s := &fakeSweep{err: errors.New("cosmos down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "subscription-sweep", nil, nil, nil, s, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on subscription sweep error", code)
	}
}

func TestRun_PgPurgeRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	p := &fakePurge{notifsPurged: 12, devicesPurged: 3}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "pg-purge", nil, nil, nil, nil, p, nil, logger)

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

	code := Run(context.Background(), "pg-purge", nil, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "pg-purge", nil, nil, nil, nil, p, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on pg-purge error", code)
	}
}

func TestRun_UnknownModeExitsOne(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "banana", nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 for unknown mode", code)
	}
}

func TestRun_AppStoreReconcileRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	r := &fakeAppStoreReconcile{scanned: 10, gaps: 2, applied: 1}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "appstore-reconcile", nil, nil, nil, nil, nil, r, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 (successful appstore-reconcile cycle)", code)
	}
	if r.calls != 1 {
		t.Errorf("appstore-reconcile Run calls: got %d, want 1", r.calls)
	}
}

// TestRun_AppStoreReconcileWithNilRunnerExitsZero pins the deliberate
// deviation from runSweep/runDormant (nil = fatal): appstore-reconcile is a
// genuinely optional feature during rollout, so unconfigured App Store Server
// API key material must log and exit 0, not crash-loop the job.
func TestRun_AppStoreReconcileWithNilRunnerExitsZero(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "appstore-reconcile", nil, nil, nil, nil, nil, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 when appstore-reconcile is unconfigured", code)
	}
	if !strings.Contains(buf.String(), "appstore-reconcile") {
		t.Errorf("log should mention appstore-reconcile, got: %s", buf.String())
	}
}

func TestRun_AppStoreReconcileCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	r := &fakeAppStoreReconcile{err: errors.New("apple unreachable")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "appstore-reconcile", nil, nil, nil, nil, nil, r, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on appstore-reconcile cycle error", code)
	}
}

// TestRun_AppStoreReconcileStampsSpanAttributes proves the scanned/gaps/applied
// counts land on the "App Store Reconcile Cycle" span so they're queryable in
// App Insights.
func TestRun_AppStoreReconcileStampsSpanAttributes(t *testing.T) {
	r := &fakeAppStoreReconcile{scanned: 7, gaps: 3, applied: 2}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	span := recordSingleSpan(t, func() {
		code := Run(context.Background(), "appstore-reconcile", nil, nil, nil, nil, nil, r, logger)
		if code != 0 {
			t.Errorf("exit code: got %d, want 0", code)
		}
	})

	if span.Name() != "App Store Reconcile Cycle" {
		t.Fatalf("span name: got %q, want %q", span.Name(), "App Store Reconcile Cycle")
	}
	if got, ok := attrInt(span, "appstore_reconcile.scanned_count"); !ok || got != 7 {
		t.Errorf("appstore_reconcile.scanned_count: got %d (ok=%v), want 7", got, ok)
	}
	if got, ok := attrInt(span, "appstore_reconcile.gaps_count"); !ok || got != 3 {
		t.Errorf("appstore_reconcile.gaps_count: got %d (ok=%v), want 3", got, ok)
	}
	if got, ok := attrInt(span, "appstore_reconcile.applied_count"); !ok || got != 2 {
		t.Errorf("appstore_reconcile.applied_count: got %d (ok=%v), want 2", got, ok)
	}
}

// TestRun_AppStoreReconcileErrorRecordsSpanError proves a runner error is
// recorded on the "App Store Reconcile Cycle" span (in addition to the exit
// code), mirroring every other cycle's error-recording behaviour.
func TestRun_AppStoreReconcileErrorRecordsSpanError(t *testing.T) {
	r := &fakeAppStoreReconcile{err: errors.New("apple unreachable")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	span := recordSingleSpan(t, func() {
		code := Run(context.Background(), "appstore-reconcile", nil, nil, nil, nil, nil, r, logger)
		if code != 1 {
			t.Errorf("exit code: got %d, want 1", code)
		}
	})

	if span.Status().Code != codes.Error {
		t.Errorf("span status code: got %v, want codes.Error", span.Status().Code)
	}
	if span.Status().Description == "" {
		t.Error("span status description: got empty, want the runner error message")
	}
}

type fakePoll struct {
	calls int
	err   error
}

func (f *fakePoll) Run(context.Context) error {
	f.calls++
	return f.err
}

func TestRun_PollModeRunsAndExitsZero(t *testing.T) {
	p := &fakePoll{}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	code := Run(context.Background(), "poll", nil, nil, p, nil, nil, nil, logger)

	if code != 0 || p.calls != 1 {
		t.Errorf("code=%d calls=%d, want 0 and 1", code, p.calls)
	}
}

func TestRun_PollModeWithoutRunnerExitsOne(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	if code := Run(context.Background(), "poll", nil, nil, nil, nil, nil, nil, logger); code != 1 {
		t.Errorf("code=%d, want 1 for an unwired poll runner", code)
	}
}

func TestRun_PollModeErrorExitsOne(t *testing.T) {
	p := &fakePoll{err: errors.New("lease store down")}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	if code := Run(context.Background(), "poll", nil, nil, p, nil, nil, nil, logger); code != 1 {
		t.Errorf("code=%d, want 1", code)
	}
}

func TestRun_RetiredModesAreUnknown(t *testing.T) {
	for _, mode := range []string{"poll-sb", "poll-bootstrap", "dev-seed"} {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		if code := Run(context.Background(), mode, nil, nil, nil, nil, nil, nil, logger); code != 1 {
			t.Errorf("mode %q: code=%d, want 1", mode, code)
		}
	}
}
