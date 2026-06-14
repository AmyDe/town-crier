package worker

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// tracerName labels the worker's OpenTelemetry spans.
const tracerName = "github.com/AmyDe/town-crier/api-go/internal/worker"

// bootstrapBudget is the soft self-cancel for a single bootstrap run. A single
// Service Bus depth probe plus an optional scheduled publish is fast; 60s is
// generous. The Container Apps replicaTimeout is the hard kill ceiling — this is
// the soft ceiling that lets the process exit cleanly and flush telemetry.
const bootstrapBudget = 60 * time.Second

// Run dispatches on WORKER_MODE and returns the process exit code. It is the
// testable core of cmd/worker/main.go — main() only loads config, wires the
// Service Bus client + bootstrapper, sets up telemetry, and propagates this
// code.
//
// Only poll-bootstrap is implemented in this skeleton. The other four modes
// (poll-sb, digest, hourly-digest, dormant-cleanup) are loud stubs that exit 1;
// the Go worker image is not deployed to any job until the final cutover bead,
// so a stub can never run in production. An unset or unknown mode is a
// deployment accident and also fails fast.
//
// bootstrapper may be nil when the job has no Service Bus config; poll-bootstrap
// then refuses to run rather than nil-panicking.
func Run(ctx context.Context, mode string, bootstrapper *Bootstrapper, logger *slog.Logger) int {
	switch mode {
	case "":
		// WORKER_MODE is always set by infra; an unset value is a deployment
		// accident — fail fast rather than silently no-op.
		logger.ErrorContext(ctx, "WORKER_MODE is unset; refusing to run")
		return 1

	case "poll-bootstrap":
		return runPollBootstrap(ctx, bootstrapper, logger)

	case "poll-sb", "digest", "hourly-digest", "dormant-cleanup":
		// Loud, safe stub: the image is not deployed until the final cutover, so
		// this exit-1 can never strand a real job. Each mode lands in its own bead.
		logger.ErrorContext(ctx, "WORKER_MODE not yet implemented in Go worker", "mode", mode)
		return 1

	default:
		logger.ErrorContext(ctx, "unknown WORKER_MODE; refusing to run", "mode", mode)
		return 1
	}
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

	// Tag names match the .NET safety-net path so existing App Insights queries
	// and dashboards keep working across the cutover.
	span.SetAttributes(
		attribute.Bool("polling.safety_net.bootstrap_published", res.Published),
		attribute.Bool("polling.safety_net.bootstrap_probe_failed", res.ProbeFailed),
	)
	return 0
}
