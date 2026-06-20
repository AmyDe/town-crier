package applications

import (
	"log/slog"
	"net/http"

	"github.com/AmyDe/town-crier/api-go/internal/httputil"
)

// writeJSON encodes v as a 200 application/json; charset=utf-8 response with HTML
// escaping off and no trailing newline.
func writeJSON(w http.ResponseWriter, r *http.Request, logger *slog.Logger, v any) {
	body, err := httputil.EncodeJSON(v)
	if err != nil {
		serverError(w, r, logger, "encode response", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	// gosec G705: body is JSON from httputil.EncodeJSON served as
	// application/json, so it is never interpreted as HTML — no XSS surface.
	if _, err := w.Write(body); err != nil { //nolint:gosec // JSON body, application/json content type
		logger.ErrorContext(r.Context(), "write response", "error", err)
	}
}

// serverError logs and emits a bodyless 500; the error envelope is backfilled by
// middleware.ErrorBody.
func serverError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, op string, err error) {
	logger.ErrorContext(r.Context(), "application request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
