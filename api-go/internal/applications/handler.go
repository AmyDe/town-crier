package applications

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

// appStore is the consumer-side store the application read handler uses.
type appStore interface {
	GetByAuthorityAndName(ctx context.Context, authorityCode, name string) (PlanningApplication, bool, error)
}

// snapshotRefresher is the consumer-side view of the saved-application
// refresh-on-tap side effect. It is declared here, not imported from
// savedapplications, because savedapplications already imports this package;
// structural typing lets *savedapplications.SnapshotRefresher satisfy it without
// an import cycle.
type snapshotRefresher interface {
	RefreshSnapshot(ctx context.Context, userID string, app PlanningApplication) error
}

// handler serves GET /v1/applications/{authorityCode}/{name}.
type handler struct {
	store     appStore
	refresher snapshotRefresher
	logger    *slog.Logger
}

// Routes registers the application read endpoint. The {name...} wildcard matches
// the remainder of the path so a PlanIt case reference containing slashes (e.g.
// "24/0123/FUL") is captured whole. The refresher is optional (nil disables
// refresh-on-tap).
func Routes(mux *http.ServeMux, store appStore, refresher snapshotRefresher, logger *slog.Logger) {
	h := &handler{store: store, refresher: refresher, logger: logger}
	mux.HandleFunc("GET /v1/applications/{authorityCode}/{name...}", h.getByAuthorityAndName)
}

// getByAuthorityAndName point-reads an application by (authorityCode, name) and
// returns it, or a bodyless 404 when it is not in Cosmos — there is no PlanIt
// fallback (GH#395 Invariant 1).
func (h *handler) getByAuthorityAndName(w http.ResponseWriter, r *http.Request) {
	authorityCode := r.PathValue("authorityCode")
	name := r.PathValue("name")

	app, found, err := h.store.GetByAuthorityAndName(r.Context(), authorityCode, name)
	if err != nil {
		serverError(w, r, h.logger, "read application", err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Refresh-on-tap: if the requesting user has this application saved, heal
	// their saved snapshot as a best-effort side effect. It must never fail the
	// read, so errors are logged and swallowed.
	if h.refresher != nil {
		if userID := auth.Subject(r.Context()); userID != "" {
			if err := h.refresher.RefreshSnapshot(r.Context(), userID, app); err != nil {
				h.logger.WarnContext(r.Context(), "refresh-on-tap failed", "user", userID, "uid", app.UID, "error", err)
			}
		}
	}

	writeJSON(w, r, h.logger, ResultOf(app))
}
