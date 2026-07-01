package applications

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/authorities"
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

// authoritySlugResolver resolves between an authority slug and its area id (which
// equals the authority id / stringified authority_code). It is the narrow
// consumer-side view of *authorities.Lookup, which satisfies it structurally.
type authoritySlugResolver interface {
	SlugToAreaID(slug string) (int, bool)
	SlugForAreaID(id int) (string, bool)
}

// handler serves the single-application read endpoints.
type handler struct {
	store     appStore
	refresher snapshotRefresher
	resolver  authoritySlugResolver
	logger    *slog.Logger
}

// Routes registers the application read endpoints. The {name...}/{ref...}
// wildcards match the remainder of the path so a PlanIt case reference containing
// slashes (e.g. "24/0123/FUL") is captured whole. The refresher is optional (nil
// disables refresh-on-tap); resolver is required (it computes authoritySlug and
// resolves the by-slug route). The literal "by-slug" segment makes that pattern
// strictly more specific than {authorityCode}, so Go 1.22's ServeMux accepts both
// without a conflict panic.
func Routes(mux *http.ServeMux, store appStore, refresher snapshotRefresher, resolver authoritySlugResolver, logger *slog.Logger) {
	h := &handler{store: store, refresher: refresher, resolver: resolver, logger: logger}
	mux.HandleFunc("GET /v1/applications/{authorityCode}/{name...}", h.getByAuthorityAndName)
	mux.HandleFunc("GET /v1/applications/by-slug/{authoritySlug}/{ref...}", h.getBySlug)
}

// getByAuthorityAndName point-reads an application by (authorityCode, name) and
// returns it, or a bodyless 404 when it is not in the store — there is no PlanIt
// fallback (GH#395 Invariant 1). This route is authed.
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

	result := ResultOf(app)
	result.AuthoritySlug = h.authoritySlug(r.Context(), app)
	writeJSON(w, r, h.logger, result)
}

// getBySlug point-reads an application by (authoritySlug, ref) and returns exactly
// the same body as the authed by-id read (including authoritySlug). It is
// ANONYMOUS and public: no auth/user data is read and refresh-on-tap never runs.
// An unknown slug or unknown ref returns a bodyless 404 (the envelope is
// backfilled by middleware.ErrorBody, same as the by-id not-found path).
func (h *handler) getBySlug(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("authoritySlug")
	ref := r.PathValue("ref")

	areaID, ok := h.resolver.SlugToAreaID(slug)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	app, found, err := h.store.GetByAuthorityAndName(r.Context(), strconv.Itoa(areaID), ref)
	if err != nil {
		serverError(w, r, h.logger, "read application by slug", err)
		return
	}
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	result := ResultOf(app)
	result.AuthoritySlug = h.authoritySlug(r.Context(), app)
	writeJSON(w, r, h.logger, result)
}

// authoritySlug returns the URL slug for the application's authority,
// round-trip-safe against SlugToAreaID: it prefers the resolver's SlugForAreaID
// (so the emitted slug is exactly what SlugToAreaID resolves back), and only when
// the id is unknown falls back to slugifying the PlanIt area name. The fallback is
// warn-logged because it means PlanIt returned an area id absent from the static
// authorities data — the emitted slug then may not round-trip back through
// SlugToAreaID, so a share/by-slug link built from it could 404.
func (h *handler) authoritySlug(ctx context.Context, app PlanningApplication) string {
	if slug, ok := h.resolver.SlugForAreaID(app.AreaID); ok {
		return slug
	}
	h.logger.WarnContext(ctx, "authority slug fallback: area id not in static authorities", "op", "authority slug", "areaId", app.AreaID, "areaName", app.AreaName, "uid", app.UID)
	return authorities.Slugify(app.AreaName)
}
