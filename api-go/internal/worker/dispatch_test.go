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

// recordSingleSpan is deliberately not parallel: it swaps the global TracerProvider.
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

func attrBool(span sdktrace.ReadOnlySpan, key string) (bool, bool) {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsBool(), true
		}
	}
	return false, false
}

func attrInt(span sdktrace.ReadOnlySpan, key string) (int64, bool) {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsInt64(), true
		}
	}
	return 0, false
}

func attrFloat64(span sdktrace.ReadOnlySpan, key string) (float64, bool) {
	for _, kv := range span.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsFloat64(), true
		}
	}
	return 0, false
}

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

type fakeDormant struct {
	calls   int
	deleted int
	err     error
}

func (f *fakeDormant) Run(context.Context) (int, error) {
	f.calls++
	return f.deleted, f.err
}

type fakeSweep struct {
	calls      int
	downgraded int
	err        error
}

func (f *fakeSweep) Run(context.Context) (int, error) {
	f.calls++
	return f.downgraded, f.err
}

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

	code := Run(context.Background(), "", nil, nil, nil, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "digest", d, nil, nil, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "hourly-digest", d, nil, nil, nil, nil, nil, nil, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0", code)
	}
	if d.hourlyCalls != 1 || d.weeklyCalls != 0 {
		t.Errorf("calls: weekly=%d hourly=%d, want 0/1", d.weeklyCalls, d.hourlyCalls)
	}
}

func TestRun_DigestModeWithoutHandlerExitsOne(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "digest", nil, nil, nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when digest handler is unconfigured", code)
	}
}

func TestRun_DigestCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	d := &fakeDigester{weeklyErr: errors.New("cosmos down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "digest", d, nil, nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on digest cycle error", code)
	}
}

func TestRun_DormantCleanupRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	d := &fakeDormant{deleted: 3}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "dormant-cleanup", nil, d, nil, nil, nil, nil, nil, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 (successful dormant cleanup)", code)
	}
	if d.calls != 1 {
		t.Errorf("dormant Run calls: got %d, want 1", d.calls)
	}
}

func TestRun_DormantCleanupWithoutHandlerExitsOne(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "dormant-cleanup", nil, nil, nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when dormant handler is unconfigured", code)
	}
}

func TestRun_DormantCleanupCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	d := &fakeDormant{err: errors.New("cosmos down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "dormant-cleanup", nil, d, nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on dormant cleanup error", code)
	}
}

func TestRun_SubscriptionSweepRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	s := &fakeSweep{downgraded: 4}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "subscription-sweep", nil, nil, nil, s, nil, nil, nil, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 (successful subscription sweep)", code)
	}
	if s.calls != 1 {
		t.Errorf("sweep Run calls: got %d, want 1", s.calls)
	}
}

func TestRun_SubscriptionSweepWithoutHandlerExitsOne(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "subscription-sweep", nil, nil, nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 when sweep handler is unconfigured", code)
	}
}

func TestRun_SubscriptionSweepCycleErrorExitsOne(t *testing.T) {
	t.Parallel()
	s := &fakeSweep{err: errors.New("cosmos down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "subscription-sweep", nil, nil, nil, s, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on subscription sweep error", code)
	}
}

func TestRun_PgPurgeRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	p := &fakePurge{notifsPurged: 12, devicesPurged: 3}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "pg-purge", nil, nil, nil, nil, p, nil, nil, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 (successful pg-purge)", code)
	}
	if p.calls != 1 {
		t.Errorf("purge Run calls: got %d, want 1", p.calls)
	}
}

func TestRun_PgPurgeWithNilRunnerExitsZero(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "pg-purge", nil, nil, nil, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "pg-purge", nil, nil, nil, nil, p, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on pg-purge error", code)
	}
}

func TestRun_UnknownModeExitsOne(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "banana", nil, nil, nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 for unknown mode", code)
	}
}

func TestRun_AppStoreReconcileRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	r := &fakeAppStoreReconcile{scanned: 10, gaps: 2, applied: 1}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "appstore-reconcile", nil, nil, nil, nil, nil, r, nil, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0 (successful appstore-reconcile cycle)", code)
	}
	if r.calls != 1 {
		t.Errorf("appstore-reconcile Run calls: got %d, want 1", r.calls)
	}
}

func TestRun_AppStoreReconcileWithNilRunnerExitsZero(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "appstore-reconcile", nil, nil, nil, nil, nil, nil, nil, nil, logger)

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

	code := Run(context.Background(), "appstore-reconcile", nil, nil, nil, nil, nil, r, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1 on appstore-reconcile cycle error", code)
	}
}

func TestRun_AppStoreReconcileStampsSpanAttributes(t *testing.T) {
	r := &fakeAppStoreReconcile{scanned: 7, gaps: 3, applied: 2}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	span := recordSingleSpan(t, func() {
		code := Run(context.Background(), "appstore-reconcile", nil, nil, nil, nil, nil, r, nil, nil, logger)
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

func TestRun_AppStoreReconcileErrorRecordsSpanError(t *testing.T) {
	r := &fakeAppStoreReconcile{err: errors.New("apple unreachable")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	span := recordSingleSpan(t, func() {
		code := Run(context.Background(), "appstore-reconcile", nil, nil, nil, nil, nil, r, nil, nil, logger)
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

	code := Run(context.Background(), "poll", nil, nil, p, nil, nil, nil, nil, nil, logger)

	if code != 0 || p.calls != 1 {
		t.Errorf("code=%d calls=%d, want 0 and 1", code, p.calls)
	}
}

func TestRun_PollModeWithoutRunnerExitsOne(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	if code := Run(context.Background(), "poll", nil, nil, nil, nil, nil, nil, nil, nil, logger); code != 1 {
		t.Errorf("code=%d, want 1 for an unwired poll runner", code)
	}
}

func TestRun_PollModeErrorExitsOne(t *testing.T) {
	p := &fakePoll{err: errors.New("lease store down")}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	if code := Run(context.Background(), "poll", nil, nil, p, nil, nil, nil, nil, nil, logger); code != 1 {
		t.Errorf("code=%d, want 1", code)
	}
}

func TestRun_RetiredModesAreUnknown(t *testing.T) {
	for _, mode := range []string{"poll-sb", "poll-bootstrap", "dev-seed"} {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, nil))
		if code := Run(context.Background(), mode, nil, nil, nil, nil, nil, nil, nil, nil, logger); code != 1 {
			t.Errorf("mode %q: code=%d, want 1", mode, code)
		}
	}
}

type fakeSEOCatalog struct {
	calls          int
	authorityPages int
	townPages      int
	err            error
}

func (f *fakeSEOCatalog) Run(context.Context) (int, int, error) {
	f.calls++
	return f.authorityPages, f.townPages, f.err
}

func TestRun_SEOAssignIsUnknownMode(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	if code := Run(context.Background(), "seo-assign", nil, nil, nil, nil, nil, nil, &fakeSEOCatalog{}, nil, logger); code != 1 {
		t.Errorf("seo-assign code=%d, want 1", code)
	}
}

func TestRun_SEOCatalogRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	r := &fakeSEOCatalog{authorityPages: 400, townPages: 1300}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "seo-catalog", nil, nil, nil, nil, nil, nil, r, nil, logger)

	if code != 0 {
		t.Errorf("exit code: got %d, want 0", code)
	}
	if r.calls != 1 {
		t.Errorf("seo-catalog Run calls: got %d, want 1", r.calls)
	}
}

func TestRun_SEOCatalogWithNilRunnerExitsOne(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	if code := Run(context.Background(), "seo-catalog", nil, nil, nil, nil, nil, nil, nil, nil, logger); code != 1 {
		t.Errorf("exit code: got %d, want 1 for a nil runner", code)
	}
}

func TestRun_SEOCatalogErrorExitsOne(t *testing.T) {
	t.Parallel()
	r := &fakeSEOCatalog{err: errors.New("collision")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	if code := Run(context.Background(), "seo-catalog", nil, nil, nil, nil, nil, nil, r, nil, logger); code != 1 {
		t.Errorf("exit code: got %d, want 1 for a run error", code)
	}
}

type fakeSEOCatalogWithIndexNow struct {
	fakeSEOCatalog
	submitCalls int
	submitted   int
	skipped     int
	submitErr   error
}

func (f *fakeSEOCatalogWithIndexNow) SubmitIndexNow(context.Context) (int, int, error) {
	f.submitCalls++
	return f.submitted, f.skipped, f.submitErr
}

func TestRun_SEOCatalogSubmitsIndexNowAfterRebuild(t *testing.T) {
	t.Parallel()
	r := &fakeSEOCatalogWithIndexNow{submitted: 5, skipped: 1715}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "seo-catalog", nil, nil, nil, nil, nil, nil, r, nil, logger)

	if code != 0 || r.calls != 1 || r.submitCalls != 1 {
		t.Errorf("code=%d rebuilds=%d submits=%d, want 0/1/1", code, r.calls, r.submitCalls)
	}
}

func TestRun_SEOCatalogSkipsIndexNowWhenRebuildFails(t *testing.T) {
	t.Parallel()
	r := &fakeSEOCatalogWithIndexNow{fakeSEOCatalog: fakeSEOCatalog{err: errors.New("collision")}}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "seo-catalog", nil, nil, nil, nil, nil, nil, r, nil, logger)

	if code != 1 || r.submitCalls != 0 {
		t.Errorf("code=%d submits=%d, want 1/0", code, r.submitCalls)
	}
}

func TestRun_SEOCatalogIndexNowFailureLogsErrorAndExitsOne(t *testing.T) {
	t.Parallel()
	r := &fakeSEOCatalogWithIndexNow{submitErr: errors.New("status 422: bad host")}
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "seo-catalog", nil, nil, nil, nil, nil, nil, r, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1", code)
	}
	if r.calls != 1 {
		t.Errorf("rebuild calls: got %d, want 1", r.calls)
	}
	out := buf.String()
	if !strings.Contains(out, `"level":"ERROR"`) || !strings.Contains(out, "status 422: bad host") {
		t.Errorf("log lacks ERROR with status/body: %s", out)
	}
}

type fakeBackup struct {
	calls  int
	tables []BackupTableStat
	err    error
}

func (f *fakeBackup) Run(context.Context) ([]BackupTableStat, error) {
	f.calls++
	return f.tables, f.err
}

func TestRun_PGBackupRunsAndExitsZero(t *testing.T) {
	t.Parallel()
	b := &fakeBackup{}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))

	code := Run(context.Background(), "pg-backup", nil, nil, nil, nil, nil, nil, nil, b, logger)

	if code != 0 || b.calls != 1 {
		t.Errorf("code=%d calls=%d, want 0/1", code, b.calls)
	}
}

func TestRun_PGBackupWithoutRunnerExitsOne(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	code := Run(context.Background(), "pg-backup", nil, nil, nil, nil, nil, nil, nil, nil, logger)

	if code != 1 {
		t.Errorf("exit code: got %d, want 1", code)
	}
	if !strings.Contains(buf.String(), "BACKUP_BLOB_URL") {
		t.Errorf("log should name the missing config, got %q", buf.String())
	}
}

func TestRun_PGBackupFailureExitsOneAndMarksSpan(t *testing.T) {
	b := &fakeBackup{err: errors.New("sink down")}
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	var code int

	span := recordSingleSpan(t, func() {
		code = Run(context.Background(), "pg-backup", nil, nil, nil, nil, nil, nil, nil, b, logger)
	})

	if code != 1 {
		t.Errorf("exit code: got %d, want 1", code)
	}
	if span.Status().Code != codes.Error {
		t.Errorf("span status: got %v, want Error", span.Status().Code)
	}
}

func TestRun_PGBackupRecordsRowAndByteCountsOnSpan(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	b := &fakeBackup{tables: []BackupTableStat{
		{Name: "users", Rows: 3, Bytes: 100},
		{Name: "watch_zones", Rows: 2, Bytes: 50},
	}}

	span := recordSingleSpan(t, func() {
		Run(context.Background(), "pg-backup", nil, nil, nil, nil, nil, nil, nil, b, logger)
	})

	if span.Name() != "Postgres Backup Cycle" {
		t.Errorf("span name = %q", span.Name())
	}
	want := map[string]int64{
		"pg_backup.users.rows":        3,
		"pg_backup.users.bytes":       100,
		"pg_backup.watch_zones.rows":  2,
		"pg_backup.watch_zones.bytes": 50,
		"pg_backup.rows_total":        5,
		"pg_backup.bytes_total":       150,
	}
	for key, w := range want {
		if got, ok := attrInt(span, key); !ok || got != w {
			t.Errorf("%s = %d (present=%v), want %d", key, got, ok, w)
		}
	}
}

type fakeApplicationLocations struct {
	loadCalls      int
	reconcileCalls int
	reconcileAll   bool
	loadErr        error
	reconcileErr   error
	repairs        int
}

func (f *fakeApplicationLocations) Load(context.Context) (int64, error) {
	f.loadCalls++
	return 7, f.loadErr
}

func (f *fakeApplicationLocations) Reconcile(_ context.Context, all bool) (int, error) {
	f.reconcileCalls++
	f.reconcileAll = all
	return f.repairs, f.reconcileErr
}

func TestIsApplicationLocationsMode(t *testing.T) {
	t.Parallel()
	for mode, want := range map[string]bool{
		"application-locations-load":          true,
		"application-locations-reconcile":     true,
		"application-locations-reconcile-all": true,
		"poll":                                false,
		"":                                    false,
	} {
		if got := IsApplicationLocationsMode(mode); got != want {
			t.Errorf("IsApplicationLocationsMode(%q) = %v, want %v", mode, got, want)
		}
	}
}

func TestRunApplicationLocations(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	cases := []struct {
		name       string
		mode       string
		fake       *fakeApplicationLocations
		nilRunner  bool
		wantCode   int
		wantLoad   int
		wantRecon  int
		wantAllArg bool
	}{
		{"load", "application-locations-load", &fakeApplicationLocations{}, false, 0, 1, 0, false},
		{"load error", "application-locations-load", &fakeApplicationLocations{loadErr: boom}, false, 1, 1, 0, false},
		{"reconcile", "application-locations-reconcile", &fakeApplicationLocations{}, false, 0, 0, 1, false},
		{"reconcile all", "application-locations-reconcile-all", &fakeApplicationLocations{}, false, 0, 0, 1, true},
		{"reconcile error", "application-locations-reconcile", &fakeApplicationLocations{reconcileErr: boom}, false, 1, 0, 1, false},
		{"no runner", "application-locations-load", nil, true, 1, 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
			var runner ApplicationLocationsRunner
			if !tc.nilRunner {
				runner = tc.fake
			}

			code := RunApplicationLocations(context.Background(), tc.mode, runner, logger)

			if code != tc.wantCode {
				t.Errorf("code = %d, want %d", code, tc.wantCode)
			}
			if tc.nilRunner {
				return
			}
			if tc.fake.loadCalls != tc.wantLoad || tc.fake.reconcileCalls != tc.wantRecon || tc.fake.reconcileAll != tc.wantAllArg {
				t.Errorf("load=%d reconcile=%d all=%v, want %d/%d/%v", tc.fake.loadCalls, tc.fake.reconcileCalls, tc.fake.reconcileAll, tc.wantLoad, tc.wantRecon, tc.wantAllArg)
			}
		})
	}
}
