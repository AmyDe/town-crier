package worker

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// WORKER_MODE values of the application_locations jobs.
const (
	ModeApplicationLocationsLoad         = "application-locations-load"
	ModeApplicationLocationsReconcile    = "application-locations-reconcile"
	ModeApplicationLocationsReconcileAll = "application-locations-reconcile-all"
)

const (
	applicationLocationsLoadBudget         = 110 * time.Minute
	applicationLocationsReconcileBudget    = 50 * time.Minute
	applicationLocationsReconcileAllBudget = 110 * time.Minute
)

// ApplicationLocationsRunner is the consumer-side slice of the thin-table
// jobs. Load returns the rows copied. Reconcile returns the rows it repaired;
// all selects every authority instead of today's bucket.
type ApplicationLocationsRunner interface {
	Load(ctx context.Context) (rows int64, err error)
	Reconcile(ctx context.Context, all bool) (repairs int, err error)
}

// IsApplicationLocationsMode reports whether mode is one of the
// application_locations jobs, which RunApplicationLocations dispatches.
func IsApplicationLocationsMode(mode string) bool {
	switch mode {
	case ModeApplicationLocationsLoad, ModeApplicationLocationsReconcile, ModeApplicationLocationsReconcileAll:
		return true
	}
	return false
}

// RunApplicationLocations runs one application_locations job and returns the
// process exit code. A nil runner or a runner error exits 1.
func RunApplicationLocations(ctx context.Context, mode string, runner ApplicationLocationsRunner, logger *slog.Logger) int {
	tracer := otel.Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "Application Locations "+mode)
	defer span.End()

	if runner == nil {
		logger.ErrorContext(ctx, "application-locations has no runner wired; refusing to run", "mode", mode)
		return 1
	}

	switch mode {
	case ModeApplicationLocationsLoad:
		runCtx, cancel := context.WithTimeout(ctx, applicationLocationsLoadBudget)
		defer cancel()
		rows, err := runner.Load(runCtx)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			logger.ErrorContext(ctx, "application-locations load failed", "error", err)
			return 1
		}
		span.SetAttributes(attribute.Int64("application_locations.loaded_rows", rows))
		logger.InfoContext(ctx, "application-locations load completed", "rows", rows)
		return 0

	case ModeApplicationLocationsReconcile, ModeApplicationLocationsReconcileAll:
		all := mode == ModeApplicationLocationsReconcileAll
		budget := applicationLocationsReconcileBudget
		if all {
			budget = applicationLocationsReconcileAllBudget
		}
		runCtx, cancel := context.WithTimeout(ctx, budget)
		defer cancel()
		repairs, err := runner.Reconcile(runCtx, all)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			logger.ErrorContext(ctx, "application-locations reconcile failed", "error", err)
			return 1
		}
		span.SetAttributes(attribute.Int("application_locations.repairs", repairs))
		return 0
	}
	logger.ErrorContext(ctx, "unknown application-locations mode", "mode", mode)
	return 1
}
