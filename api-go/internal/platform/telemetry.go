// telemetry.go wires OpenTelemetry tracing and logging for the Go API. The
// go-coding-standards skill favours a minimal dependency tree, but GH#418 (Go
// API it10) and ADR 0027 mandate OpenTelemetry so the Go app emits OTLP traces
// AND logs to the Azure Container Apps managed-environment OTel agent, which
// forwards them to Application Insights. This leaves the .NET app's in-process
// Azure Monitor exporter untouched, so the two implementations don't
// double-count during the cutover window.
//
// tc-8x8g adds the logs pipeline: the four new deps below
// (otelslog / otlploggrpc / sdk/log / log/global, pinned to the otel v1.44.0
// release line) bridge slog records to OTel logs so they land in App Insights
// AppTraces. Without them the Go API emitted zero AppTraces and AppExceptions,
// leaving 500s undiagnosable.
//
// tc-21np adds the metrics pipeline: an otlpmetricgrpc exporter behind an SDK
// MeterProvider (PeriodicReader), installed as the global meter provider, plus
// Go runtime metrics via contrib/instrumentation/runtime. This restores the
// towncrier.* business metrics (defined in internal/metrics) and runtime /
// http.client metrics that went dark at the .NET->Go cutover. There is NO direct
// Azure Monitor exporter — like traces and logs, metrics export OTLP/gRPC to the
// ACA managed-environment OTel agent, which forwards them to App Insights
// AppMetrics.
package platform

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// defaultServiceName labels traces when OTEL_SERVICE_NAME is unset.
const defaultServiceName = "town-crier-api-go"

// cosmosDatabasePrefix is the shared prefix on the Cosmos database names
// (town-crier-prod / town-crier-dev); the suffix is the deployment environment.
const cosmosDatabasePrefix = "town-crier-"

// deploymentEnvironment derives the deployment.environment resource attribute
// value from the COSMOS_DATABASE name. "town-crier-prod" -> "prod",
// "town-crier-dev" -> "dev". An empty database (local dev, tests) or any name
// without the expected "town-crier-" prefix returns "" so the caller omits the
// attribute rather than emitting deployment.environment="".
func deploymentEnvironment(cosmosDB string) string {
	if cosmosDB == "" {
		return ""
	}
	env, ok := strings.CutPrefix(cosmosDB, cosmosDatabasePrefix)
	if !ok {
		return ""
	}
	return env
}

// newTelemetryResource builds the OTel resource shared by the trace, log and
// metric providers. service.name is set explicitly (Azure Monitor maps it to
// AppRoleName); it deliberately keeps the current town-crier-{api,worker}-go
// role names so existing Go-role dashboards keep matching. deployment.environment
// is added from the COSMOS_DATABASE name so prod and dev signal separate as a
// dimension instead of commingling under one role (tc-yqyh); it is omitted
// entirely when the environment can't be derived (local dev, tests). WithFromEnv
// is applied last so an infra-injected OTEL_RESOURCE_ATTRIBUTES (e.g.
// deployment.environment=prod) can override these explicit values without a code
// change.
func newTelemetryResource(ctx context.Context, serviceName, cosmosDB string) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{semconv.ServiceName(serviceName)}
	if env := deploymentEnvironment(cosmosDB); env != "" {
		attrs = append(attrs, semconv.DeploymentEnvironment(env))
	}
	return resource.New(ctx,
		resource.WithAttributes(attrs...),
		resource.WithFromEnv(),
	)
}

// noopShutdown is returned when telemetry is disabled — there is nothing to flush.
func noopShutdown(context.Context) error { return nil }

// SetupTelemetry configures the global OpenTelemetry tracer, logger and meter
// providers.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is unset or empty (local dev, tests, the
// contract suite, or a Cosmos-less boot) it self-disables: no exporters are
// built, the global no-op TracerProvider, LoggerProvider and MeterProvider are
// left in place, and a no-op shutdown is returned. This keeps every existing
// test green with no infra-then-test split.
//
// When the endpoint is set (injected by the ACA OTel agent) it builds OTLP/gRPC
// trace, log AND metric exporters — otlptracegrpc.New / otlploggrpc.New /
// otlpmetricgrpc.New read the endpoint and related standard OTEL_* env vars
// themselves and dial lazily, so this never blocks even if the collector is
// unreachable — installs SDK TracerProvider, LoggerProvider and MeterProvider as
// the global providers (sharing one resource), sets a W3C TraceContext
// propagator, starts Go runtime metrics against the new MeterProvider, and
// returns a combined Shutdown that flushes all three providers.
func SetupTelemetry(ctx context.Context, logger *slog.Logger) (func(context.Context) error, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		logger.InfoContext(ctx, "telemetry disabled", "reason", "OTEL_EXPORTER_OTLP_ENDPOINT unset")
		return noopShutdown, nil
	}

	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = defaultServiceName
	}
	res, err := newTelemetryResource(ctx, serviceName, os.Getenv("COSMOS_DATABASE"))
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	logExporter, err := otlploggrpc.New(ctx)
	if err != nil {
		// Tear the tracer provider down so a half-built pipeline doesn't leak.
		if shutdownErr := tp.Shutdown(ctx); shutdownErr != nil {
			logger.ErrorContext(ctx, "tracer shutdown after log-exporter failure", "error", shutdownErr)
		}
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)
	global.SetLoggerProvider(lp)

	metricExporter, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		// Tear the already-built providers down so a half-built pipeline doesn't
		// leak, mirroring the tracer-teardown-on-log-failure path above.
		if shutdownErr := tp.Shutdown(ctx); shutdownErr != nil {
			logger.ErrorContext(ctx, "tracer shutdown after metric-exporter failure", "error", shutdownErr)
		}
		if shutdownErr := lp.Shutdown(ctx); shutdownErr != nil {
			logger.ErrorContext(ctx, "logger shutdown after metric-exporter failure", "error", shutdownErr)
		}
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	// Start Go runtime metrics (GC, goroutines, memory) against the new provider.
	// http.client metrics come from the otelhttp transport (tc-166z) using this
	// same global MeterProvider; the towncrier.* business metrics come from
	// internal/metrics built on otel.Meter("towncrier"). A runtime.Start failure
	// is non-fatal — runtime metrics are a nice-to-have, not worth aborting the
	// whole telemetry pipeline (and the rest of the metrics still flow).
	if rerr := runtime.Start(runtime.WithMeterProvider(mp)); rerr != nil {
		logger.ErrorContext(ctx, "runtime metrics start failed; continuing without them", "error", rerr)
	}

	logger.InfoContext(ctx, "telemetry enabled", "endpoint", endpoint, "service", serviceName)

	// Combined shutdown flushes/shuts down all three providers, returning the
	// first error so no flush is skipped.
	return func(shutdownCtx context.Context) error {
		errTrace := tp.Shutdown(shutdownCtx)
		errLog := lp.Shutdown(shutdownCtx)
		errMetric := mp.Shutdown(shutdownCtx)
		switch {
		case errTrace != nil:
			return errTrace
		case errLog != nil:
			return errLog
		default:
			return errMetric
		}
	}, nil
}
