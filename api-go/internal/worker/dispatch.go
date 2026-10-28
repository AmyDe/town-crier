package worker

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const tracerName = "github.com/AmyDe/town-crier/api-go/internal/worker"

// Each budget is the soft self-cancel for one worker cycle and must stay at or
// below 80% of the job's replicaTimeout in infra/environment.go, so the process
// exits cleanly and flushes telemetry. cycleBudget covers the 600 s jobs.
const (
	cycleBudget        = 8 * time.Minute
	digestBudget       = 8 * time.Minute
	hourlyDigestBudget = 4 * time.Minute
)

// seoBudget stays under the seo-catalog job's 1,800 s replicaTimeout.
const seoBudget = 28 * time.Minute

// DigestRunner is the digest handler as the dispatcher sees it. It is exported
// so main() can pass a genuinely nil interface; a typed-nil *digest.Handler
// would defeat the nil guard.
type DigestRunner interface {
	RunWeekly(ctx context.Context) error
	RunHourly(ctx context.Context) error
}

// DormantRunner is exported for the same typed-nil reason as DigestRunner.
// Run returns the number of accounts erased.
type DormantRunner interface {
	Run(ctx context.Context) (int, error)
}

// SweepRunner is exported for the same typed-nil reason as DigestRunner.
// Run returns the number of profiles downgraded.
type SweepRunner interface {
	Run(ctx context.Context) (int, error)
}

// PurgeRunner's Run returns the notification and device-registration rows
// deleted. A nil runner is a deliberate no-op: dispatch logs and exits 0.
type PurgeRunner interface {
	Run(ctx context.Context) (notifsPurged int, devicesPurged int, err error)
}

// PollRunner is the consumer-side slice of the poll runner the dispatcher
// invokes. Run takes the polling lease, works until its own run budget is spent
// and returns an error only for failures the run cannot absorb (lease or state
// store errors); PlanIt limits end a run normally.
type PollRunner interface {
	Run(ctx context.Context) error
}

// AppStoreReconcileRunner's Run returns the scanned, gap and applied counts.
// Unlike SweepRunner and DormantRunner, a nil runner is a deliberate non-fatal
// no-op: unconfigured App Store key material must not crash-loop the job.
type AppStoreReconcileRunner interface {
	Run(ctx context.Context) (scanned, gaps, applied int, err error)
}

// SEOCatalogRunner is the consumer-side slice of the SEO catalog rebuild job.
// Run returns the number of authority and town pages published.
type SEOCatalogRunner interface {
	Run(ctx context.Context) (authorityPages, townPages int, err error)
}

// IndexNowSubmitter is optionally implemented by a SEOCatalogRunner that should
// notify IndexNow after a successful rebuild. It returns the URLs submitted and
// the unchanged URLs skipped.
type IndexNowSubmitter interface {
	SubmitIndexNow(ctx context.Context) (submitted, skipped int, err error)
}

// BackupTableStat is one table's row and compressed byte count in a backup.
type BackupTableStat struct {
	Name  string
	Rows  int64
	Bytes int64
}

// BackupRunner is exported for the same typed-nil reason as DigestRunner.
// Run returns per-table counts for a completed backup.
type BackupRunner interface {
	Run(ctx context.Context) ([]BackupTableStat, error)
}

// Run dispatches on WORKER_MODE and returns the process exit code. An unset or unknown mode is a deployment
// accident and fails fast. Optional runners (purger, reconciler) may be nil:
// pg-purge and appstore-reconcile then log and exit 0; the others, including
// pg-backup, exit 1.
func Run(ctx context.Context, mode string, digester DigestRunner, dormant DormantRunner, poller PollRunner, sweeper SweepRunner, purger PurgeRunner, reconciler AppStoreReconcileRunner, seoCataloger SEOCatalogRunner, backuper BackupRunner, logger *slog.Logger) int {
	switch mode {
	case "":
		logger.ErrorContext(ctx, "WORKER_MODE is unset; refusing to run")
		return 1

	case "digest":
		return runDigest(ctx, "Digest Cycle", digestBudget, digester, DigestRunner.RunWeekly, logger)

	case "hourly-digest":
		return runDigest(ctx, "Hourly Digest Cycle", hourlyDigestBudget, digester, DigestRunner.RunHourly, logger)

	case "dormant-cleanup":
		return runDormant(ctx, dormant, logger)

	case "subscription-sweep":
		return runSweep(ctx, sweeper, logger)

	case "poll":
		return runPoll(ctx, poller, logger)

	case "pg-purge":
		return runPurge(ctx, purger, logger)

	case "appstore-reconcile":
		return runAppStoreReconcile(ctx, reconciler, logger)

	case "seo-catalog":
		return runSEOCatalog(ctx, seoCataloger, logger)

	case "pg-backup":
		return runBackup(ctx, backuper, logger)

	default:
		logger.ErrorContext(ctx, "unknown WORKER_MODE; refusing to run", "mode", mode)
		return 1
	}
}

// runPoll executes one hourly poll run. The runner opens its own "PlanIt poll
// run" span and reports health on it, so a PlanIt limit exits 0. A nil runner
// (unwired job) or a runner error exits 1.
func runPoll(ctx context.Context, poller PollRunner, logger *slog.Logger) int {
	if poller == nil {
		logger.ErrorContext(ctx, "poll requires PlanIt and Postgres config; refusing to run")
		return 1
	}
	if err := poller.Run(ctx); err != nil {
		logger.ErrorContext(ctx, "poll run failed", "error", err)
		return 1
	}
	return 0
}

// runDigest keeps the span names "Digest Cycle" and "Hourly Digest Cycle" so
// existing App Insights queries keep working.
func runDigest(ctx context.Context, spanName string, budget time.Duration, digester DigestRunner, run func(DigestRunner, context.Context) error, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, spanName)
	defer span.End()

	if digester == nil {
		logger.ErrorContext(ctx, "digest mode requires Cosmos + ACS/APNs config (COSMOS_ENDPOINT et al.); refusing to run", "span", spanName)
		return 1
	}

	cycleCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	if err := run(digester, cycleCtx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.ErrorContext(ctx, "digest cycle failed", "span", spanName, "error", err)
		return 1
	}
	return 0
}

// runDormant keeps the span name "Dormant Cleanup Cycle" so existing App
// Insights queries keep working.
func runDormant(ctx context.Context, runner DormantRunner, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Dormant Cleanup Cycle")
	defer span.End()

	if runner == nil {
		logger.ErrorContext(ctx, "dormant-cleanup requires Cosmos config (COSMOS_ENDPOINT et al.); refusing to run")
		return 1
	}

	cycleCtx, cancel := context.WithTimeout(ctx, cycleBudget)
	defer cancel()

	deleted, err := runner.Run(cycleCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.ErrorContext(ctx, "dormant cleanup cycle failed", "error", err)
		return 1
	}
	span.SetAttributes(attribute.Int("dormant_cleanup.deleted_count", deleted))
	return 0
}

func runSweep(ctx context.Context, runner SweepRunner, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Subscription Sweep Cycle")
	defer span.End()

	if runner == nil {
		logger.ErrorContext(ctx, "subscription-sweep requires Cosmos config (COSMOS_ENDPOINT et al.); refusing to run")
		return 1
	}

	cycleCtx, cancel := context.WithTimeout(ctx, cycleBudget)
	defer cancel()

	downgraded, err := runner.Run(cycleCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.ErrorContext(ctx, "subscription sweep cycle failed", "error", err)
		return 1
	}
	span.SetAttributes(attribute.Int("subscription_sweep.downgraded_count", downgraded))
	return 0
}

// runPurge treats a nil runner as a deliberate no-op (exit 0), not a deployment error.
func runPurge(ctx context.Context, runner PurgeRunner, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Postgres Purge Cycle")
	defer span.End()

	if runner == nil {
		logger.InfoContext(ctx, "pg-purge: store backend is not postgres; Cosmos TTL handles expiry")
		return 0
	}

	cycleCtx, cancel := context.WithTimeout(ctx, cycleBudget)
	defer cancel()

	notifsPurged, devicesPurged, err := runner.Run(cycleCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.ErrorContext(ctx, "pg-purge cycle failed", "error", err)
		return 1
	}
	span.SetAttributes(
		attribute.Int("pg_purge.notifications_deleted", notifsPurged),
		attribute.Int("pg_purge.device_registrations_deleted", devicesPurged),
	)
	logger.InfoContext(ctx, "pg-purge cycle completed",
		"notificationsDeleted", notifsPurged,
		"deviceRegistrationsDeleted", devicesPurged)
	return 0
}

// runAppStoreReconcile treats a nil runner as a deliberate no-op (exit 0):
// unconfigured App Store key material must not crash-loop the job.
func runAppStoreReconcile(ctx context.Context, runner AppStoreReconcileRunner, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "App Store Reconcile Cycle")
	defer span.End()

	if runner == nil {
		logger.InfoContext(ctx, "appstore-reconcile unconfigured (APPSTORE_RECONCILE_ENABLED / key material unset); skipping")
		return 0
	}

	cycleCtx, cancel := context.WithTimeout(ctx, cycleBudget)
	defer cancel()

	scanned, gaps, applied, err := runner.Run(cycleCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.ErrorContext(ctx, "appstore-reconcile cycle failed", "error", err)
		return 1
	}
	span.SetAttributes(
		attribute.Int("appstore_reconcile.scanned_count", scanned),
		attribute.Int("appstore_reconcile.gaps_count", gaps),
		attribute.Int("appstore_reconcile.applied_count", applied),
	)
	logger.InfoContext(ctx, "appstore-reconcile cycle completed",
		"scanned", scanned, "gaps", gaps, "applied", applied)
	return 0
}

// runSEOCatalog calls IndexNowSubmitter after a successful rebuild; its failure
// exits 1 without undoing the catalog, so the job shows red and the next run resubmits.
func runSEOCatalog(ctx context.Context, runner SEOCatalogRunner, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "SEO Catalog Rebuild")
	defer span.End()

	if runner == nil {
		logger.ErrorContext(ctx, "seo-catalog has no runner wired; refusing to run")
		return 1
	}

	runCtx, cancel := context.WithTimeout(ctx, seoBudget)
	defer cancel()

	authorityPages, townPages, err := runner.Run(runCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.ErrorContext(ctx, "seo-catalog run failed", "error", err)
		return 1
	}
	span.SetAttributes(
		attribute.Int("seo_catalog.authority_pages", authorityPages),
		attribute.Int("seo_catalog.town_pages", townPages),
	)

	submitter, ok := runner.(IndexNowSubmitter)
	if !ok {
		return 0
	}
	submitted, skipped, err := submitter.SubmitIndexNow(runCtx)
	span.SetAttributes(
		attribute.Int("seo_catalog.indexnow_submitted", submitted),
		attribute.Int("seo_catalog.indexnow_skipped", skipped),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.ErrorContext(ctx, "seo-catalog indexnow submission failed", "error", err)
		return 1
	}
	return 0
}

// runBackup fails closed: a nil runner means BACKUP_BLOB_URL or
// BACKUP_CONTAINER was unset, and a silent skip would hide missing backups.
func runBackup(ctx context.Context, runner BackupRunner, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Postgres Backup Cycle")
	defer span.End()

	if runner == nil {
		logger.ErrorContext(ctx, "pg-backup requires BACKUP_BLOB_URL and BACKUP_CONTAINER; refusing to run")
		return 1
	}

	cycleCtx, cancel := context.WithTimeout(ctx, cycleBudget)
	defer cancel()

	tables, err := runner.Run(cycleCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.ErrorContext(ctx, "pg-backup cycle failed", "error", err)
		return 1
	}

	var rows, bytes int64
	for _, t := range tables {
		rows += t.Rows
		bytes += t.Bytes
		span.SetAttributes(
			attribute.Int64("pg_backup."+t.Name+".rows", t.Rows),
			attribute.Int64("pg_backup."+t.Name+".bytes", t.Bytes),
		)
	}
	span.SetAttributes(
		attribute.Int64("pg_backup.rows_total", rows),
		attribute.Int64("pg_backup.bytes_total", bytes),
	)
	logger.InfoContext(ctx, "pg-backup cycle completed", "tables", len(tables), "rows", rows, "bytes", bytes)
	return 0
}
