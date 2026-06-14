// telemetry.go wires OpenTelemetry tracing for the Go API. The go-coding-standards
// skill favours a minimal dependency tree, but GH#418 (Go API it10) mandates
// OpenTelemetry so the Go app emits OTLP traces to the Azure Container Apps
// managed-environment OTel agent, which forwards them to Application Insights.
// This leaves the .NET app's in-process Azure Monitor exporter untouched, so the
// two implementations don't double-count during the cutover window.
package platform

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// defaultServiceName labels traces when OTEL_SERVICE_NAME is unset.
const defaultServiceName = "town-crier-api-go"

// noopShutdown is returned when telemetry is disabled — there is nothing to flush.
func noopShutdown(context.Context) error { return nil }

// SetupTelemetry configures the global OpenTelemetry tracer provider.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is unset or empty (local dev, tests, the
// contract suite, or a Cosmos-less boot) it self-disables: no exporter is built,
// the global no-op TracerProvider is left in place, and a no-op shutdown is
// returned. This keeps every existing test green with no infra-then-test split.
//
// When the endpoint is set (injected by the ACA OTel agent) it builds an
// OTLP/gRPC trace exporter — otlptracegrpc.New reads the endpoint and related
// standard OTEL_* env vars itself and dials lazily, so this never blocks even if
// the collector is unreachable — installs an SDK TracerProvider as the global
// provider, sets a W3C TraceContext propagator, and returns the provider's
// Shutdown as the cleanup func.
func SetupTelemetry(ctx context.Context, logger *slog.Logger) (func(context.Context) error, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		logger.InfoContext(ctx, "telemetry disabled", "reason", "OTEL_EXPORTER_OTLP_ENDPOINT unset")
		return noopShutdown, nil
	}

	exporter, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
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

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	logger.InfoContext(ctx, "telemetry enabled", "endpoint", endpoint, "service", serviceName)
	return tp.Shutdown, nil
}
