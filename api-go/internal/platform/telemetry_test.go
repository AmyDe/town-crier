package platform

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// restoreOTelGlobals captures the process-global TracerProvider and propagator
// and reinstates them when the test ends, so a test that installs a real SDK
// provider doesn't bleed into the rest of the suite.
func restoreOTelGlobals(t *testing.T) {
	t.Helper()
	tp := otel.GetTracerProvider()
	prop := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(prop)
	})
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
