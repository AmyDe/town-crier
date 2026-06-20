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

// Run dispatches on WORKER_MODE and returns the process exit code. It is the
// testable core of cmd/worker/main.go — main() only loads config, wires the
// Service Bus client + bootstrapper, sets up telemetry, and propagates this
// code.
//
// poll-bootstrap, digest, hourly-digest, and dormant-cleanup are implemented;
// poll-sb remains a loud stub that exits 1 until its own bead (tc-yng2) lands.
// The Go worker image is not deployed to any job until the final cutover bead, so
// a stub can never run in production. An unset or unknown mode is a deployment
// accident and also fails fast.
//
// bootstrapper may be nil when the job has no Service Bus config; poll-bootstrap
// then refuses to run rather than nil-panicking. Likewise digester / dormant may
// be nil when the job has no Cosmos config; those modes then refuse to run rather
// than nil-panicking.
func Run(ctx context.Context, mode string, bootstrapper *Bootstrapper, digester DigestRunner, dormant DormantRunner, poller PollOrchestrator, logger *slog.Logger) int {
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

	case "poll-sb":
		return runPollSB(ctx, poller, logger)

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
