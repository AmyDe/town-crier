package applications

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
)

// appStore is the consumer-side store the application read handler uses.
type appStore interface {
	GetByAuthorityAndName(ctx context.Context, authorityCode, name string) (PlanningApplication, bool, error)
}

// handler serves GET /v1/applications/{authorityCode}/{name}.
type handler struct {
	store  appStore
	logger *slog.Logger
}

// Routes registers the application read endpoint. The {name...} wildcard matches
// the remainder of the path so a PlanIt case reference containing slashes (e.g.
// "24/0123/FUL") is captured whole, mirroring the .NET {**name} catch-all.
func Routes(mux *http.ServeMux, store appStore, logger *slog.Logger) {
	h := &handler{store: store, logger: logger}
	mux.HandleFunc("GET /v1/applications/{authorityCode}/{name...}", h.getByAuthorityAndName)
}

// getByAuthorityAndName point-reads an application by (authorityCode, name) and
// returns it, or a bodyless 404 when it is not in Cosmos — there is no PlanIt
// fallback (GH#395 Invariant 1). Refresh-on-tap (the saved-snapshot heal side
// effect) is deferred to bead tc-wans.
func (h *handler) getByAuthorityAndName(w http.ResponseWriter, r *http.Request) {
	authorityCode := r.PathValue("authorityCode")
	name := r.PathValue("name")

	app, found, err := h.store.GetByAuthorityAndName(r.Context(), authorityCode, name)
	if err != nil {
		h.serverError(w, r, "read application", err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	h.writeJSON(w, r, resultFrom(app))
}

// writeJSON encodes v as a 200 application/json; charset=utf-8 response with HTML
// escaping off and no trailing newline, matching ASP.NET's Results.Ok byte output.
func (h *handler) writeJSON(w http.ResponseWriter, r *http.Request, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		h.serverError(w, r, "encode response", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(bytes.TrimRight(buf.Bytes(), "\n")); err != nil {
		h.logger.ErrorContext(r.Context(), "write response", "error", err)
	}
}

// serverError logs and emits a bodyless 500; the error envelope is backfilled by
// middleware.ErrorBody.
func (h *handler) serverError(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logger.ErrorContext(r.Context(), "application request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
