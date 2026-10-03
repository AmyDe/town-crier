package worker

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// tracerName labels the worker's OpenTelemetry spans.
const tracerName = "github.com/AmyDe/town-crier/api-go/internal/worker"

// digestBudget is the soft self-cancel for a single digest / hourly-digest run.
// A digest cycle fans out across many users' Cosmos reads and email/push sends;
// 10 minutes is generous while still bounded well under the Container Apps
// replicaTimeout so the process exits cleanly and flushes telemetry.
const digestBudget = 10 * time.Minute

// dormantBudget is the soft self-cancel for a single dormant-cleanup run. The
// cycle scans all profiles cross-partition then runs a per-account erasure
// cascade for the (small) dormant set; 10 minutes is generous while still bounded
// well under the Container Apps replicaTimeout so the process exits cleanly and
// flushes telemetry.
const dormantBudget = 10 * time.Minute

// sweepBudget is the soft self-cancel for a single subscription-sweep run. The
// cycle scans all profiles cross-partition then downgrades the (small) lapsed-paid
// set with a Cosmos upsert and an Auth0 PATCH each; 10 minutes is generous while
// still bounded well under the Container Apps replicaTimeout so the process exits
// cleanly and flushes telemetry.
const sweepBudget = 10 * time.Minute

// purgeBudget is the soft self-cancel for a single pg-purge run. A DELETE
// WHERE created_at < cutoff on two tables is fast even at scale; 10 minutes is
// generous while still bounded well under the Container Apps replicaTimeout so
// the process exits cleanly and flushes telemetry.
const purgeBudget = 10 * time.Minute

// reconcileBudget is the soft self-cancel for a single appstore-reconcile run.
// The cycle is one paginated App Store Server API call plus, in apply mode, a
// handful of NotificationProcessor.Process calls for the (small) gap set; 10
// minutes is generous while still bounded well under the Container Apps
// replicaTimeout so the process exits cleanly and flushes telemetry.
const reconcileBudget = 10 * time.Minute

// DigestRunner is the consumer-side slice of the digest handler the dispatcher
// invokes. *digest.Handler satisfies it; the worker depends only on these two
// methods so it need not know the handler's internals. It is exported so main()
// can hold a genuinely nil interface value when the job has no digest config —
// passing a typed-nil *digest.Handler would defeat the nil guard below.
type DigestRunner interface {
	RunWeekly(ctx context.Context) error
	RunHourly(ctx context.Context) error
}

// DormantRunner is the consumer-side slice of the dormant-cleanup handler the
// dispatcher invokes. *dormant.Handler satisfies it; Run returns the number of
// accounts erased so the dispatcher can record it as a telemetry tag. It is
// exported so main() can hold a genuinely nil interface value when the job has no
// Cosmos config — passing a typed-nil *dormant.Handler would defeat the nil guard.
type DormantRunner interface {
	Run(ctx context.Context) (int, error)
}

// SweepRunner is the consumer-side slice of the subscription-sweep handler the
// dispatcher invokes. *subscriptionsweep.Handler satisfies it; Run returns the
// number of profiles downgraded so the dispatcher can record it as a telemetry
// tag. It is exported so main() can hold a genuinely nil interface value when the
// job has no Cosmos config — passing a typed-nil *subscriptionsweep.Handler would
// defeat the nil guard.
type SweepRunner interface {
	Run(ctx context.Context) (int, error)
}

// PurgeRunner is the consumer-side slice of the pg-purge handler the dispatcher
// invokes. *pgpurge.Handler satisfies it; Run returns the number of notification
// rows and device-registration rows deleted so the dispatcher can record them as
// telemetry tags. It is exported so main() can hold a genuinely nil interface
// value when no purge runner is configured — the nil runner causes dispatch to
// log and exit 0 (an unconfigured pg-purge is a deliberate no-op, not a
// deployment error).
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

// AppStoreReconcileRunner is the consumer-side slice of the appstore-reconcile
// handler the dispatcher invokes. *appstorereconcile.Handler satisfies it;
// Run returns the scanned/gap/applied counts so the dispatcher can record
// them as telemetry tags. It is exported so main() can hold a genuinely nil
// interface value when the job is missing its App Store Server API key
// material — passing a typed-nil *appstorereconcile.Handler would defeat the
// nil guard below. Unlike SweepRunner/DormantRunner (nil = fatal), a nil
// AppStoreReconcileRunner is a deliberate, non-fatal no-op: this feature is
// genuinely optional during rollout, and unconfigured key material must not
// crash-loop the job.
type AppStoreReconcileRunner interface {
	Run(ctx context.Context) (scanned, gaps, applied int, err error)
}

// Run dispatches on WORKER_MODE and returns the process exit code. It is the
// testable core of cmd/worker/main.go. An unset or unknown mode is a deployment
// accident and fails fast. Optional runners (purger, reconciler) may be nil:
// pg-purge and appstore-reconcile then log and exit 0; the others exit 1.
func Run(ctx context.Context, mode string, digester DigestRunner, dormant DormantRunner, poller PollRunner, sweeper SweepRunner, purger PurgeRunner, reconciler AppStoreReconcileRunner, logger *slog.Logger) int {
	switch mode {
	case "":
		// WORKER_MODE is always set by infra; an unset value is a deployment
		// accident — fail fast rather than silently no-op.
		logger.ErrorContext(ctx, "WORKER_MODE is unset; refusing to run")
		return 1

	case "digest":
		return runDigest(ctx, "Digest Cycle", digester, DigestRunner.RunWeekly, logger)

	case "hourly-digest":
		return runDigest(ctx, "Hourly Digest Cycle", digester, DigestRunner.RunHourly, logger)

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

// runDigest executes one digest cycle (weekly or hourly) under a soft self-cancel
// budget, inside a telemetry span ("Digest Cycle" / "Hourly Digest Cycle") so
// existing App Insights queries keep working. A nil digester (job missing
// Cosmos/ACS config) is an exit-1 condition; a cycle error is recorded on the
// span and also exits 1 so the job surfaces the failure.
func runDigest(ctx context.Context, spanName string, digester DigestRunner, run func(DigestRunner, context.Context) error, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, spanName)
	defer span.End()

	if digester == nil {
		logger.ErrorContext(ctx, "digest mode requires Cosmos + ACS/APNs config (COSMOS_ENDPOINT et al.); refusing to run", "span", spanName)
		return 1
	}

	cycleCtx, cancel := context.WithTimeout(ctx, digestBudget)
	defer cancel()

	if err := run(digester, cycleCtx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.ErrorContext(ctx, "digest cycle failed", "span", spanName, "error", err)
		return 1
	}
	return 0
}

// runDormant executes one dormant-cleanup cycle under a soft self-cancel budget,
// inside a telemetry span named "Dormant Cleanup Cycle" (so existing App Insights
// queries keep working). It records the number of erased accounts as the
// dormant_cleanup.deleted_count tag. A nil runner (job missing Cosmos config) is
// an exit-1 condition; a cycle error is recorded on the span and also exits 1 so
// the job surfaces the failure.
func runDormant(ctx context.Context, runner DormantRunner, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Dormant Cleanup Cycle")
	defer span.End()

	if runner == nil {
		logger.ErrorContext(ctx, "dormant-cleanup requires Cosmos config (COSMOS_ENDPOINT et al.); refusing to run")
		return 1
	}

	cycleCtx, cancel := context.WithTimeout(ctx, dormantBudget)
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

// runSweep executes one subscription-sweep cycle under a soft self-cancel budget,
// inside a telemetry span named "Subscription Sweep Cycle". It records the number
// of downgraded profiles as the subscription_sweep.downgraded_count tag. A nil
// runner (job missing Cosmos config) is an exit-1 condition; a cycle error is
// recorded on the span and also exits 1 so the job surfaces the failure.
func runSweep(ctx context.Context, runner SweepRunner, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Subscription Sweep Cycle")
	defer span.End()

	if runner == nil {
		logger.ErrorContext(ctx, "subscription-sweep requires Cosmos config (COSMOS_ENDPOINT et al.); refusing to run")
		return 1
	}

	cycleCtx, cancel := context.WithTimeout(ctx, sweepBudget)
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

// runPurge executes one pg-purge cycle under a soft self-cancel budget, inside a
// telemetry span named "Postgres Purge Cycle". It tags the span with the count of
// notification and device-registration rows deleted. A nil runner exits 0 with a
// log — an unconfigured pg-purge is a deliberate no-op, not a deployment error.
// A non-nil runner error exits 1 so the job surfaces the failure.
func runPurge(ctx context.Context, runner PurgeRunner, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Postgres Purge Cycle")
	defer span.End()

	if runner == nil {
		logger.InfoContext(ctx, "pg-purge: store backend is not postgres; Cosmos TTL handles expiry")
		return 0
	}

	cycleCtx, cancel := context.WithTimeout(ctx, purgeBudget)
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

// runAppStoreReconcile executes one appstore-reconcile cycle under a soft
// self-cancel budget, inside a telemetry span named "App Store Reconcile
// Cycle". It tags the span with the scanned/gap/applied counts. Unlike
// runSweep/runDormant (nil runner = fatal), a nil runner here logs and exits 0
// — mirroring runPurge's "backend doesn't apply here" pattern — because this
// is a genuinely optional feature during rollout: unconfigured App Store
// Server API key material must not crash-loop the job. A non-nil runner error
// (Apple unreachable, retries exhausted) exits 1 so the job surfaces the
// failure.
func runAppStoreReconcile(ctx context.Context, runner AppStoreReconcileRunner, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "App Store Reconcile Cycle")
	defer span.End()

	if runner == nil {
		logger.InfoContext(ctx, "appstore-reconcile unconfigured (APPSTORE_RECONCILE_ENABLED / key material unset); skipping")
		return 0
	}

	cycleCtx, cancel := context.WithTimeout(ctx, reconcileBudget)
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
