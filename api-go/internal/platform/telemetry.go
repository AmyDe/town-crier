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
package platform

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// defaultServiceName labels traces when OTEL_SERVICE_NAME is unset.
const defaultServiceName = "town-crier-api-go"

// noopShutdown is returned when telemetry is disabled — there is nothing to flush.
func noopShutdown(context.Context) error { return nil }

// SetupTelemetry configures the global OpenTelemetry tracer and logger providers.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is unset or empty (local dev, tests, the
// contract suite, or a Cosmos-less boot) it self-disables: no exporters are
// built, the global no-op TracerProvider and LoggerProvider are left in place,
// and a no-op shutdown is returned. This keeps every existing test green with no
// infra-then-test split.
//
// When the endpoint is set (injected by the ACA OTel agent) it builds OTLP/gRPC
// trace AND log exporters — otlptracegrpc.New / otlploggrpc.New read the
// endpoint and related standard OTEL_* env vars themselves and dial lazily, so
// this never blocks even if the collector is unreachable — installs SDK
// TracerProvider and LoggerProvider as the global providers (sharing one
// resource), sets a W3C TraceContext propagator, and returns a combined Shutdown
// that flushes both providers.
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
	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
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

	logger.InfoContext(ctx, "telemetry enabled", "endpoint", endpoint, "service", serviceName)

	// Combined shutdown flushes/shuts down both providers, returning the first
	// error so neither flush is skipped.
	return func(shutdownCtx context.Context) error {
		errTrace := tp.Shutdown(shutdownCtx)
		errLog := lp.Shutdown(shutdownCtx)
		if errTrace != nil {
			return errTrace
		}
		return errLog
	}, nil
}
