// Package health serves the liveness endpoints. They return a static body
// without touching any dependency: Container Apps' readiness probe gates
// traffic on them, so any work here delays the first request after a
// scale-from-zero cold start.
package health

import (
	"log/slog"
	"net/http"
)

// body is the .NET HealthStatus contract verbatim: {"status":"Healthy"}.
// Kept as a pre-encoded constant so the handler does no work per request.
const body = `{"status":"Healthy"}`

// Routes registers the health endpoints. The .NET API exposes the check at
// both the bare and the /v1 path; parity requires both.
func Routes(mux *http.ServeMux, logger *slog.Logger) {
	h := handler{logger: logger}
	mux.HandleFunc("GET /health", h.check)
	mux.HandleFunc("GET /v1/health", h.check)
}

type handler struct {
	logger *slog.Logger
}

func (h handler) check(w http.ResponseWriter, r *http.Request) {
	// Content-Type is application/json including the explicit charset parameter.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write([]byte(body)); err != nil {
		h.logger.ErrorContext(r.Context(), "write health response", "error", err)
	}
}
