package platform

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log/global"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func TestDeploymentEnvironment(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		postgresDB string
		want       string
	}{
		{"prod database", "town_crier_prod", "prod"},
		{"dev database", "town_crier_dev", "dev"},
		{"empty database", "", ""},
		{"unprefixed name omits environment", "somethingelse", ""},
		{"prefix only yields empty env", "town_crier_", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := deploymentEnvironment(tc.postgresDB); got != tc.want {
				t.Errorf("deploymentEnvironment(%q) = %q, want %q", tc.postgresDB, got, tc.want)
			}
		})
	}
}

// restoreOTelGlobals captures the process-global TracerProvider, propagator,
// LoggerProvider and MeterProvider and reinstates them when the test ends, so a
// test that installs a real SDK provider doesn't bleed into the rest of the
// suite.
func restoreOTelGlobals(t *testing.T) {
	t.Helper()
	tp := otel.GetTracerProvider()
	prop := otel.GetTextMapPropagator()
	lp := global.GetLoggerProvider()
	mp := otel.GetMeterProvider()
	t.Cleanup(func() {
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(prop)
		global.SetLoggerProvider(lp)
		otel.SetMeterProvider(mp)
	})
}

// resourceHasAttr reports whether the resource carries an attribute with the
// given key, and returns its string value.
func resourceHasAttr(res *resource.Resource, key attribute.Key) (string, bool) {
	for _, kv := range res.Attributes() {
		if kv.Key == key {
			return kv.Value.AsString(), true
		}
	}
	return "", false
}

func TestNewTelemetryResource_TagsDeploymentEnvironment(t *testing.T) {
	res, err := newTelemetryResource(context.Background(), "town-crier-api-go", "town_crier_prod")
	if err != nil {
		t.Fatalf("newTelemetryResource: %v", err)
	}

	const envKey = attribute.Key("deployment.environment")
	got, ok := resourceHasAttr(res, envKey)
	if !ok {
		t.Fatalf("resource missing %q attribute; attributes: %v", envKey, res.Attributes())
	}
	if got != "prod" {
		t.Errorf("deployment.environment = %q, want %q", got, "prod")
	}

	// service.name must remain explicitly set and unchanged.
	svc, ok := resourceHasAttr(res, semconv.ServiceNameKey)
	if !ok || svc != "town-crier-api-go" {
		t.Errorf("service.name = %q (present=%v), want %q", svc, ok, "town-crier-api-go")
	}
}

func TestNewTelemetryResource_OmitsEnvironmentWhenPostgresDBUnset(t *testing.T) {
	res, err := newTelemetryResource(context.Background(), "town-crier-api-go", "")
	if err != nil {
		t.Fatalf("newTelemetryResource: %v", err)
	}

	const envKey = attribute.Key("deployment.environment")
	if got, ok := resourceHasAttr(res, envKey); ok {
		t.Errorf("expected no deployment.environment attribute when POSTGRES_DB unset, got %q", got)
	}

	// service.name must still be present.
	if svc, ok := resourceHasAttr(res, semconv.ServiceNameKey); !ok || svc != "town-crier-api-go" {
		t.Errorf("service.name = %q (present=%v), want %q", svc, ok, "town-crier-api-go")
	}
}

func TestSetupTelemetry_DisabledWhenEndpointUnset(t *testing.T) {
	restoreOTelGlobals(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	before := otel.GetTracerProvider()

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	shutdown, err := SetupTelemetry(context.Background(), logger)
	if err != nil {
		t.Fatalf("SetupTelemetry: unexpected error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("SetupTelemetry: shutdown func is nil")
	}

	// The global TracerProvider must be untouched — still the SDK's no-op default,
	// not one of our SDK providers.
	if got := otel.GetTracerProvider(); got != before {
		t.Errorf("global TracerProvider was replaced while disabled: got %T", got)
	}
	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
		t.Error("an SDK TracerProvider was installed while telemetry is disabled")
	}
	// The global LoggerProvider must likewise stay the no-op default, so bridged
	// slog records are dropped (only stdout JSON appears).
	if _, ok := global.GetLoggerProvider().(*sdklog.LoggerProvider); ok {
		t.Error("an SDK LoggerProvider was installed while telemetry is disabled")
	}
	// The global MeterProvider must stay the no-op default so towncrier.*
	// instruments record nothing locally (tc-21np).
	if _, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider); ok {
		t.Error("an SDK MeterProvider was installed while telemetry is disabled")
	}

	if err := shutdown(context.Background()); err != nil {
		t.Errorf("no-op shutdown returned error: %v", err)
	}

	if !strings.Contains(buf.String(), "telemetry disabled") {
		t.Errorf("expected a 'telemetry disabled' info line, got: %s", buf.String())
	}
}

func TestSetupTelemetry_EnabledWhenEndpointSet(t *testing.T) {
	restoreOTelGlobals(t)
	// A dummy endpoint that nothing listens on. otlptracegrpc dials lazily, so
	// setup must not block even though the endpoint is unreachable.
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	t.Setenv("OTEL_SERVICE_NAME", "town-crier-api-go")

	var buf strings.Builder
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	done := make(chan struct{})
	var (
		shutdown func(context.Context) error
		setupErr error
	)
	go func() {
		shutdown, setupErr = SetupTelemetry(context.Background(), logger)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SetupTelemetry blocked with an unreachable endpoint; it must dial lazily")
	}

	if setupErr != nil {
		t.Fatalf("SetupTelemetry: unexpected error: %v", setupErr)
	}
	if shutdown == nil {
		t.Fatal("SetupTelemetry: shutdown func is nil")
	}

	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
		t.Errorf("expected an SDK TracerProvider installed as global, got %T", otel.GetTracerProvider())
	}
	// SetupTelemetry must also install an SDK LoggerProvider so slog records
	// bridge to OTel logs (-> AppTraces) — tc-8x8g.
	if _, ok := global.GetLoggerProvider().(*sdklog.LoggerProvider); !ok {
		t.Errorf("expected an SDK LoggerProvider installed as global, got %T", global.GetLoggerProvider())
	}
	// SetupTelemetry must also install an SDK MeterProvider so towncrier.*
	// instruments (and Go runtime + http.client metrics) flow to AppMetrics —
	// tc-21np.
	if _, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider); !ok {
		t.Errorf("expected an SDK MeterProvider installed as global, got %T", otel.GetMeterProvider())
	}

	// Shutdown must be callable without hanging even though no collector exists.
	shutdownDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		shutdownDone <- shutdown(ctx)
	}()
	select {
	case <-shutdownDone:
		// Any result (nil or a deadline error) is acceptable; what matters is it
		// returns rather than hanging.
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown hung")
	}
}
