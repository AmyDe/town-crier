// Package versionconfig serves GET /v1/version-config — the minimum client
// version gate. It is anonymous and returns a static value, so the handler
// does no work per request beyond writing a pre-encoded body.
package versionconfig

import (
	"log/slog"
	"net/http"
)

// body is the exact wire response: {"minimumVersion":"1.0.0"} in camelCase with
// no whitespace.
const body = `{"minimumVersion":"1.0.0"}`

// Routes registers the version-config endpoint on mux.
func Routes(mux *http.ServeMux, logger *slog.Logger) {
	h := handler{logger: logger}
	mux.HandleFunc("GET /v1/version-config", h.get)
}

type handler struct {
	logger *slog.Logger
}

func (h handler) get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write([]byte(body)); err != nil {
		h.logger.ErrorContext(r.Context(), "write version-config response", "error", err)
	}
}
