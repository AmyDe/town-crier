package demoaccount

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
)

// writeJSON encodes v as a 200 application/json; charset=utf-8 response with HTML
// escaping off and no trailing newline, matching ASP.NET's Results.Ok byte output.
func writeJSON(w http.ResponseWriter, r *http.Request, logger *slog.Logger, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		serverError(w, r, logger, "encode response", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(bytes.TrimRight(buf.Bytes(), "\n")); err != nil {
		logger.ErrorContext(r.Context(), "write response", "error", err)
	}
}

// serverError logs and emits a bodyless 500; the error envelope is backfilled by
// middleware.ErrorBody.
func serverError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, op string, err error) {
	logger.ErrorContext(r.Context(), "demo-account request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
