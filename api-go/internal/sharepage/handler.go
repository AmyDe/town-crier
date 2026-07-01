// Package sharepage serves the public, anonymous, server-rendered HTML share
// page for a single planning application at GET /a/{authoritySlug}/{ref...} —
// the tracer surface of the shareable-application-page epic (#738).
//
// The page emits ONLY public planning data: it reads no auth or user data, sets
// no cookies, and logs no client IP. Its identity is the (authority slug, PlanIt
// ref) pair; the ref is a trailing wildcard because PlanIt case references
// contain slashes (e.g. "23/03456/FUL"). Rendering uses html/template, whose
// contextual auto-escaping is a hard requirement here: the application fields
// come from an external provider and are untrusted.
package sharepage

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// appStore is the narrow consumer-side view of the applications store: a single
// point read by (authorityCode, ref). The authorityCode is the stringified area
// id. *applications.PostgresStore satisfies it structurally.
type appStore interface {
	GetByAuthorityAndName(ctx context.Context, authorityCode, name string) (applications.PlanningApplication, bool, error)
}

// slugResolver resolves an authority slug to its area id (which equals the
// authority id / stringified authority_code). *authorities.Lookup satisfies it
// structurally.
type slugResolver interface {
	SlugToAreaID(slug string) (int, bool)
}

type handler struct {
	store    appStore
	resolver slugResolver
	logger   *slog.Logger
}

// Routes registers the anonymous share-page route. The {ref...} wildcard captures
// the remainder of the path so a PlanIt reference containing slashes is matched
// whole. Keep the pattern in lockstep with the anonymousPatterns entry in
// cmd/api/wiring.go — the auth middleware keys on the exact pattern string.
func Routes(mux *http.ServeMux, store appStore, resolver slugResolver, logger *slog.Logger) {
	h := &handler{store: store, resolver: resolver, logger: logger}
	mux.HandleFunc("GET /a/{authoritySlug}/{ref...}", h.serve)
}

// serve resolves the authority slug, point-reads the application, and renders the
// page. An unknown slug or unknown ref renders the branded 404 (never a 500); a
// store error is a bodyless 500 (the envelope is backfilled by middleware).
func (h *handler) serve(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("authoritySlug")
	ref := r.PathValue("ref")

	areaID, ok := h.resolver.SlugToAreaID(slug)
	if !ok {
		h.renderNotFound(w, r)
		return
	}

	app, found, err := h.store.GetByAuthorityAndName(r.Context(), strconv.Itoa(areaID), ref)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "share page read failed", "op", "read application by slug", "slug", slug, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !found {
		h.renderNotFound(w, r)
		return
	}

	h.render(w, r, http.StatusOK, "public, max-age=3600", "page", buildPageView(app, slug, ref))
}

// renderNotFound emits the branded 404. It must NOT carry the 200's
// max-age=3600: a just-ingested application would otherwise be negatively cached
// at the edge for an hour, so the 404 is no-store.
func (h *handler) renderNotFound(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusNotFound, "no-store", "notfound", nil)
}

// render executes the named template into a buffer first, so a mid-render error
// becomes a clean bodyless 500 rather than a half-written page, then writes the
// status, content type and cache header.
func (h *handler) render(w http.ResponseWriter, r *http.Request, status int, cacheControl, name string, data any) {
	var buf bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		h.logger.ErrorContext(r.Context(), "share page render failed", "template", name, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", cacheControl)
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		h.logger.ErrorContext(r.Context(), "share page write failed", "template", name, "error", err)
	}
}
