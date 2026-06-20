package platform

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
)

// NewOTelLogger returns a logger that fans every record out to BOTH a stdout
// JSON handler (-> ContainerAppConsoleLogs) AND the otelslog bridge (-> OTel
// logs -> App Insights AppTraces). tc-8x8g / ADR 0027: without the bridge the Go
// API emits zero AppTraces, so error-level slog records never reach telemetry.
//
// The otelslog handler reads the GLOBAL OTel LoggerProvider, which is the no-op
// provider until SetupTelemetry installs an SDK one. So when telemetry is
// disabled, the bridge silently drops records and only the JSON sink emits —
// preserving the existing behaviour. The bridge auto-attaches trace/span IDs
// from the ctx passed to slog's *Context methods, giving trace correlation for
// free.
func NewOTelLogger(w io.Writer, level slog.Level) *slog.Logger {
	jsonHandler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	otelHandler := otelslog.NewHandler(defaultServiceName)
	return slog.New(newFanoutHandler(level, jsonHandler, otelHandler))
}

// fanoutHandler is a leveled multi-handler: the stdlib has no built-in way to
// send one slog record to several handlers. It gates on a single configured
// level and forwards Handle/WithAttrs/WithGroup to every child so both sinks
// receive the same attributes and groups.
type fanoutHandler struct {
	level    slog.Leveler
	children []slog.Handler
}

func newFanoutHandler(level slog.Leveler, children ...slog.Handler) *fanoutHandler {
	return &fanoutHandler{level: level, children: children}
}

func (h *fanoutHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, c := range h.children {
		if !c.Enabled(ctx, r.Level) {
			continue
		}
		if err := c.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (h *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(h.children))
	for i, c := range h.children {
		next[i] = c.WithAttrs(attrs)
	}
	return &fanoutHandler{level: h.level, children: next}
}

func (h *fanoutHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(h.children))
	for i, c := range h.children {
		next[i] = c.WithGroup(name)
	}
	return &fanoutHandler{level: h.level, children: next}
}
