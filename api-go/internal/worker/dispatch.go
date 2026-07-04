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

// bootstrapBudget is the soft self-cancel for a single bootstrap run. A single
// Service Bus depth probe plus an optional scheduled publish is fast; 60s is
// generous. The Container Apps replicaTimeout is the hard kill ceiling — this is
// the soft ceiling that lets the process exit cleanly and flush telemetry.
const bootstrapBudget = 60 * time.Second

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

// devSeedBudget is the soft self-cancel for a single dev-seed run. The cycle
// reads dev's watched authorities, pulls at most DEV_SEED_LIMIT
// recently-changed prod applications over a second, read-only pool, and feeds
// each through the same upsert + decision-dispatch + zone-enqueue + push-flush
// pipeline poll-sb uses; 5 minutes is generous for a handful of applications
// while still bounded well under the Container Apps replicaTimeout so the
// process exits cleanly and flushes telemetry.
const devSeedBudget = 5 * time.Minute

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

// PollRunResult is the dispatcher-facing summary of one poll-sb cycle. It mirrors
// the orchestrator's run result plus the ingestion counts the dispatch span tags
// and the exit-code logic need, decoupling the worker package from the polling
// package's concrete result types. The cmd/worker adapter flattens
// polling.OrchestratorRunResult into this shape.
type PollRunResult struct {
	MessageReceived   bool
	PublishedNext     bool
	LeaseUnavailable  bool
	ApplicationCount  int
	AuthoritiesPolled int
	AuthorityErrors   int
	Termination       string
}

// PollOrchestrator is the consumer-side slice of the poll-sb orchestrator the
// dispatcher invokes. The cmd/worker adapter over *polling.Orchestrator satisfies
// it. It is exported so main() can hold a genuinely nil interface value when the
// job has no Service Bus / Cosmos config — poll-sb then refuses to run rather
// than nil-panicking.
type PollOrchestrator interface {
	RunOnce(ctx context.Context) (PollRunResult, error)
}

// DevSeedRunner is the consumer-side slice of the dev-seed job the dispatcher
// invokes. *devseed.Seeder satisfies it; Run returns the number of applications
// ingested so the dispatcher can record it as a telemetry tag. It is exported so
// main() can hold a genuinely nil interface value when the job is missing its
// dedicated prod-read config (DEV_SEED_PROD_AZURE_CLIENT_ID /
// DEV_SEED_PROD_POSTGRES_USER) — passing a typed-nil *devseed.Seeder would
// defeat the nil guard below.
type DevSeedRunner interface {
	Run(ctx context.Context) (int, error)
}

// Run dispatches on WORKER_MODE and returns the process exit code. It is the
// testable core of cmd/worker/main.go — main() only loads config, wires the
// Service Bus client + bootstrapper, sets up telemetry, and propagates this
// code.
//
// poll-bootstrap, digest, hourly-digest, dormant-cleanup, subscription-sweep,
// and pg-purge are implemented; poll-sb remains a loud stub that exits 1 until
// its own bead (tc-yng2) lands. The Go worker image is not deployed to any job
// until the final cutover bead, so a stub can never run in production. An unset
// or unknown mode is a deployment accident and also fails fast.
//
// bootstrapper may be nil when the job has no Service Bus config; poll-bootstrap
// then refuses to run rather than nil-panicking. purger may be nil when no purge
// runner is configured; pg-purge then logs and exits 0, so this is never an
// error. devSeeder may be nil when the job is missing its dedicated prod-read
// config; dev-seed then refuses to run rather than nil-panicking (it is a
// dev-only job — tc-grvu.6 — so this never fires in prod).
func Run(ctx context.Context, mode string, bootstrapper *Bootstrapper, digester DigestRunner, dormant DormantRunner, poller PollOrchestrator, sweeper SweepRunner, purger PurgeRunner, devSeeder DevSeedRunner, logger *slog.Logger) int {
	switch mode {
	case "":
		// WORKER_MODE is always set by infra; an unset value is a deployment
		// accident — fail fast rather than silently no-op.
		logger.ErrorContext(ctx, "WORKER_MODE is unset; refusing to run")
		return 1

	case "poll-bootstrap":
		return runPollBootstrap(ctx, bootstrapper, logger)

	case "digest":
		return runDigest(ctx, "Digest Cycle", digester, DigestRunner.RunWeekly, logger)

	case "hourly-digest":
		return runDigest(ctx, "Hourly Digest Cycle", digester, DigestRunner.RunHourly, logger)

	case "dormant-cleanup":
		return runDormant(ctx, dormant, logger)

	case "subscription-sweep":
		return runSweep(ctx, sweeper, logger)

	case "poll-sb":
		return runPollSB(ctx, poller, logger)

	case "pg-purge":
		return runPurge(ctx, purger, logger)

	case "dev-seed":
		return runDevSeed(ctx, devSeeder, logger)

	default:
		logger.ErrorContext(ctx, "unknown WORKER_MODE; refusing to run", "mode", mode)
		return 1
	}
}

// runPollSB executes one Service-Bus-triggered adaptive poll cycle inside a
// telemetry span named "Polling Cycle (SB)" (so existing App Insights queries
// keep working). It tags the span with the canonical keys
// (polling.sb.message_received / published_next / authorities_polled /
// applications_ingested / termination / authority_errors) and applies the
// exit-code rule: exit 1 only when the run did NO useful work AND hit authority
// errors. A nil orchestrator (job missing Service Bus / Cosmos config) is an
// exit-1 condition; an orchestrator error is recorded on the span and also exits 1.
func runPollSB(ctx context.Context, poller PollOrchestrator, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Polling Cycle (SB)")
	defer span.End()

	if poller == nil {
		logger.ErrorContext(ctx, "poll-sb requires Service Bus + Cosmos config (SERVICE_BUS_* / COSMOS_*); refusing to run")
		return 1
	}

	res, err := poller.RunOnce(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.ErrorContext(ctx, "poll-sb cycle failed", "error", err)
		return 1
	}

	span.SetAttributes(
		attribute.Bool("polling.sb.message_received", res.MessageReceived),
		attribute.Bool("polling.sb.published_next", res.PublishedNext),
		attribute.Int("polling.authorities_polled", res.AuthoritiesPolled),
		attribute.Int("polling.applications_ingested", res.ApplicationCount),
		attribute.Int("polling.authority_errors", res.AuthorityErrors),
		attribute.String("polling.termination", res.Termination),
	)

	logger.InfoContext(ctx, "poll-sb cycle completed",
		"applicationsIngested", res.ApplicationCount,
		"authoritiesPolled", res.AuthoritiesPolled,
		"authorityErrors", res.AuthorityErrors,
		"leaseUnavailable", res.LeaseUnavailable)

	// Exit-code rule: only exit 1 when the run did no useful work AND hit
	// authority errors. A quiet cycle (0 apps, 0 errors), a lease-unavailable
	// exit, and any cycle that ingested apps all exit 0.
	if res.ApplicationCount == 0 && res.AuthorityErrors > 0 {
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

// runDevSeed executes one dev-seed cycle under a soft self-cancel budget,
// inside a telemetry span named "Dev Seed Cycle". It records the number of
// applications ingested as the dev_seed.ingested_count tag. A nil runner (job
// missing its dedicated prod-read config, DEV_SEED_PROD_AZURE_CLIENT_ID /
// DEV_SEED_PROD_POSTGRES_USER) is an exit-1 condition, mirroring
// dormant-cleanup/subscription-sweep's posture for an unconfigured optional
// job — dev-seed is created dev-only (tc-grvu.6), so this never fires in prod.
// A cycle error is recorded on the span and also exits 1 so the job surfaces
// the failure.
func runDevSeed(ctx context.Context, runner DevSeedRunner, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Dev Seed Cycle")
	defer span.End()

	if runner == nil {
		logger.ErrorContext(ctx, "dev-seed requires prod-read config (DEV_SEED_PROD_AZURE_CLIENT_ID / DEV_SEED_PROD_POSTGRES_USER); refusing to run")
		return 1
	}

	cycleCtx, cancel := context.WithTimeout(ctx, devSeedBudget)
	defer cancel()

	ingested, err := runner.Run(cycleCtx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		logger.ErrorContext(ctx, "dev-seed cycle failed", "error", err)
		return 1
	}
	span.SetAttributes(attribute.Int("dev_seed.ingested_count", ingested))
	return 0
}

// runPollBootstrap executes one bootstrap cycle under a soft self-cancel budget,
// recording the safety-net telemetry tags. A probe/publish failure is absorbed
// by the bootstrapper (the next cron tick retries), so it does not fail the job;
// only a missing Service Bus client (nil bootstrapper) is an exit-1 condition.
func runPollBootstrap(ctx context.Context, bootstrapper *Bootstrapper, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Polling Bootstrap")
	defer span.End()

	if bootstrapper == nil {
		logger.ErrorContext(ctx, "poll-bootstrap requires Service Bus config (SERVICE_BUS_NAMESPACE / SERVICE_BUS_QUEUE_NAME); refusing to run")
		return 1
	}

	cycleCtx, cancel := context.WithTimeout(ctx, bootstrapBudget)
	defer cancel()

	res, err := bootstrapper.TryBootstrap(cycleCtx)
	if err != nil {
		// TryBootstrap absorbs Service Bus failures itself; a returned error is a
		// caller-side concern (e.g. context cancelled).
		span.SetAttributes(attribute.Bool("polling.safety_net.bootstrap_probe_failed", true))
		logger.ErrorContext(ctx, "poll-bootstrap cycle failed", "error", err)
		return 1
	}

	// Tag names match the App Insights telemetry schema so existing queries
	// and dashboards keep working.
	span.SetAttributes(
		attribute.Bool("polling.safety_net.bootstrap_published", res.Published),
		attribute.Bool("polling.safety_net.bootstrap_probe_failed", res.ProbeFailed),
	)
	return 0
}
