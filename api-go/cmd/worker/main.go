// Command worker runs the Town Crier background-worker modes as short-lived
// Container Apps Jobs — the Go port of the .NET town-crier.worker (GH#418
// Phase 2, epic tc-wad3). One process per job: WORKER_MODE selects the mode,
// the process runs it once, flushes telemetry, and exits with a status code.
//
// This skeleton implements only poll-bootstrap (the Service-Bus-only tracer
// bullet); the other four modes are loud stubs that exit 1 until their own beads
// land. The Go image is not deployed to any job until the final cutover, so a
// stub can never strand a real job.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/servicebus"
	"github.com/AmyDe/town-crier/api-go/internal/worker"
)

func main() {
	os.Exit(run())
}

// run is main's body split out so its deferred telemetry flush executes before
// the process exits — os.Exit in main would skip every defer. It returns the
// process exit code, propagated by main via os.Exit.
func run() int {
	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Print(err)
		return 1
	}

	logger := platform.NewLogger(os.Stdout, cfg.LogLevel)

	mode := os.Getenv("WORKER_MODE")

	// SetupTelemetry self-disables when OTEL_EXPORTER_OTLP_ENDPOINT is unset, so
	// local/dev boots leave the no-op TracerProvider in place. OTEL_SERVICE_NAME
	// (set to town-crier-worker-go by infra) drives the service name on exported
	// spans. The deferred shutdown force-flushes the final batch before this
	// short-lived process exits — without it the worker can terminate before its
	// spans reach the collector (mirrors the .NET worker's ForceFlush).
	shutdownTelemetry, err := platform.SetupTelemetry(context.Background(), logger)
	if err != nil {
		log.Print(err)
		return 1
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdownTelemetry(shutdownCtx); err != nil {
			logger.Error("telemetry shutdown", "error", err)
		}
	}()

	// The Service Bus client (and thus the bootstrapper) is built only when the
	// job carries Service Bus config. Jobs that don't touch Service Bus (digest,
	// hourly-digest, dormant-cleanup) leave the bootstrapper nil; poll-bootstrap
	// then refuses to run rather than crashing.
	var bootstrapper *worker.Bootstrapper
	if cfg.ServiceBusNamespace != "" && cfg.ServiceBusQueueName != "" {
		sbClient, err := servicebus.NewClient(cfg.ServiceBusNamespace, cfg.ServiceBusQueueName, cfg.AzureClientID)
		if err != nil {
			logger.Error("build service bus client", "error", err)
			return 1
		}
		defer func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := sbClient.Close(closeCtx); err != nil {
				logger.Error("service bus client close", "error", err)
			}
		}()
		bootstrapper = worker.NewBootstrapper(sbClient, logger, time.Now)
	}

	return worker.Run(context.Background(), mode, bootstrapper, logger)
}
