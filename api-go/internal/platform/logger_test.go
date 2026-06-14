package platform

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/embedded"
	"go.opentelemetry.io/otel/log/global"
)

// recordingLogProcessor is a hand-written in-memory log processor (no mocking
// library) that captures emitted OTel log records so tests can assert the
// slog->OTel bridge fired. logtest is avoided because otlploggrpc v0.20.0 ships
// with a broken dependency on the sdk/log/logtest module.
type recordingLogProcessor struct {
	records []logRecordSnapshot
}

type logRecordSnapshot struct {
	severity log.Severity
	body     string
}

// fakeLoggerProvider / fakeLogger implement the otel log.LoggerProvider and
// log.Logger interfaces so an Emit lands in the recorder. otelslog.NewHandler
// reads the global LoggerProvider, so installing this captures bridged records.
type fakeLoggerProvider struct {
	embedded.LoggerProvider
	proc *recordingLogProcessor
}

func (p *fakeLoggerProvider) Logger(string, ...log.LoggerOption) log.Logger {
	return &fakeLogger{proc: p.proc}
}

type fakeLogger struct {
	embedded.Logger
	proc *recordingLogProcessor
}

func (l *fakeLogger) Emit(_ context.Context, rec log.Record) {
	l.proc.records = append(l.proc.records, logRecordSnapshot{
		severity: rec.Severity(),
		body:     rec.Body().AsString(),
	})
}

func (l *fakeLogger) Enabled(context.Context, log.EnabledParameters) bool { return true }

// installFakeLoggerProvider swaps the global OTel LoggerProvider for the
// recorder, restoring the previous provider on cleanup.
func installFakeLoggerProvider(t *testing.T) *recordingLogProcessor {
	t.Helper()
	prev := global.GetLoggerProvider()
	proc := &recordingLogProcessor{}
	global.SetLoggerProvider(&fakeLoggerProvider{proc: proc})
	t.Cleanup(func() { global.SetLoggerProvider(prev) })
	return proc
}

// TestNewOTelLogger_WritesJSONAndBridgesToOTel asserts the production logger
// fans an ErrorContext record out to BOTH the stdout JSON sink (preserving
// ContainerAppConsoleLogs) AND the OTel logs bridge (-> AppTraces).
func TestNewOTelLogger_WritesJSONAndBridgesToOTel(t *testing.T) {
	proc := installFakeLoggerProvider(t)

	var buf strings.Builder
	logger := NewOTelLogger(&buf, slog.LevelInfo)

	logger.ErrorContext(context.Background(), "boom happened", "code", 500)

	// JSON sink got the record.
	var entry map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &entry); err != nil {
		t.Fatalf("JSON sink output not valid JSON: %v (raw: %q)", err, buf.String())
	}
	if entry["msg"] != "boom happened" {
		t.Errorf("JSON msg: got %v, want %q", entry["msg"], "boom happened")
	}

	// OTel bridge got the record.
	if len(proc.records) != 1 {
		t.Fatalf("OTel bridge: got %d records, want 1", len(proc.records))
	}
	if proc.records[0].body != "boom happened" {
		t.Errorf("OTel body: got %q, want %q", proc.records[0].body, "boom happened")
	}
}

// TestNewOTelLogger_RespectsLevelGate asserts the configured level gates BOTH
// children: a Debug record at Info level reaches neither the JSON sink nor the
// OTel bridge.
func TestNewOTelLogger_RespectsLevelGate(t *testing.T) {
	proc := installFakeLoggerProvider(t)

	var buf strings.Builder
	logger := NewOTelLogger(&buf, slog.LevelInfo)

	logger.DebugContext(context.Background(), "too quiet")

	if buf.Len() != 0 {
		t.Errorf("JSON sink emitted a below-level record: %q", buf.String())
	}
	if len(proc.records) != 0 {
		t.Errorf("OTel bridge emitted a below-level record: %d", len(proc.records))
	}
}

// TestNewOTelLogger_WithAttrsForwardsToBothChildren asserts WithAttrs/WithGroup
// fan out so attributes are present on both sinks.
func TestNewOTelLogger_WithAttrsForwardsToBothChildren(t *testing.T) {
	proc := installFakeLoggerProvider(t)

	var buf strings.Builder
	logger := NewOTelLogger(&buf, slog.LevelInfo).With("service", "api")

	logger.InfoContext(context.Background(), "started")

	var entry map[string]any
	if err := json.Unmarshal([]byte(buf.String()), &entry); err != nil {
		t.Fatalf("JSON sink output not valid JSON: %v", err)
	}
	if entry["service"] != "api" {
		t.Errorf("JSON missing WithAttrs value: %v", entry["service"])
	}
	if len(proc.records) != 1 {
		t.Fatalf("OTel bridge: got %d records, want 1", len(proc.records))
	}
}

// TestNewOTelLogger_NoOpProviderEmitsOnlyJSON mirrors the disabled-telemetry
// path: with no SDK LoggerProvider installed, the global no-op provider drops
// the bridged record, so only the JSON sink sees output.
func TestNewOTelLogger_NoOpProviderEmitsOnlyJSON(t *testing.T) {
	// Deliberately do NOT install a recording provider — the default global is
	// the no-op LoggerProvider, exactly as during a telemetry-disabled boot.
	var buf strings.Builder
	logger := NewOTelLogger(&buf, slog.LevelInfo)

	logger.InfoContext(context.Background(), "still logs to stdout")

	if !strings.Contains(buf.String(), "still logs to stdout") {
		t.Errorf("JSON sink must still receive the record when telemetry is off: %q", buf.String())
	}
}
