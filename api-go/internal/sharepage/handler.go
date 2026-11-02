// Package sharepage serves the public, anonymous, server-rendered HTML share
// page for a single planning application at GET /a/{authoritySlug}/{ref...}.
//
// The page emits ONLY public planning data: it reads no auth or user data, sets
// no cookies, and logs no client IP. Its identity is the (authority slug, PlanIt
// ref) pair; the ref is a trailing wildcard because PlanIt case references
// contain slashes (e.g. "23/03456/FUL").
package sharepage

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/sharelink"
)

// appStore point-reads an application by (authorityCode, ref); authorityCode is
// the stringified area id.
type appStore interface {
	GetByAuthorityAndName(ctx context.Context, authorityCode, name string) (applications.PlanningApplication, bool, error)
}

// slugResolver resolves an authority slug to its area id (which equals the
// authority id / stringified authority_code).
type slugResolver interface {
	SlugToAreaID(slug string) (int, bool)
}

// clickRecorder counts a likely-human visit without doing I/O.
type clickRecorder interface {
	Add(slug, ref string, source SourceGroup)
}

type handler struct {
	store    appStore
	resolver slugResolver
	clicks   clickRecorder
	logger   *slog.Logger
}

// Routes registers the anonymous share page and its fixed App Store redirect.
// The {ref...} wildcard captures the remainder of the path so a PlanIt reference
// containing slashes is matched whole. Keep the patterns in lockstep with the
// anonymousPatterns entries in cmd/api/wiring.go — the auth middleware keys on
// the exact pattern string. clicks may be nil, which turns visit counting off.
func Routes(mux *http.ServeMux, store appStore, resolver slugResolver, clicks clickRecorder, logger *slog.Logger) {
	h := &handler{store: store, resolver: resolver, clicks: clicks, logger: logger}
	mux.HandleFunc("GET /a/{authoritySlug}/{ref...}", h.serve)
	mux.HandleFunc("GET "+appStoreRedirectPath, appStoreRedirect)
}

// appStoreRedirect reads no destination from the request, so it cannot act as an
// open redirect.
func appStoreRedirect(w http.ResponseWriter, r *http.Request) {
	sharelink.Record(r)
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, ctaURL(), http.StatusFound)
}

// serve renders the branded 404 (never a 500) for an unknown slug or ref; a store
// error is a bodyless 500 (the envelope is backfilled by middleware).
func (h *handler) serve(w http.ResponseWriter, r *http.Request) {
	code := sharelink.Record(r)
	slug := r.PathValue("authoritySlug")
	ref := r.PathValue("ref")

	areaID, ok := h.resolver.SlugToAreaID(slug)
	if !ok {
		h.renderNotFound(w, r)
		return
	}

	app, found, err := h.store.GetByAuthorityAndName(r.Context(), strconv.Itoa(areaID), ref)
	if err != nil {
		platform.RespondRequestError(w, r, h.logger, "share page read failed", err, "op", "read application by slug", "slug", slug, "ref", ref)
		return
	}
	if !found {
		h.renderNotFound(w, r)
		return
	}

	if h.render(w, r, http.StatusOK, "public, max-age=3600", "page", buildPageView(app, slug, ref).withShareCode(code)) {
		h.countVisit(r, slug, ref)
	}
}

// The 404 is no-store: carrying the 200's max-age would negatively cache a
// just-ingested application at the edge for an hour.
func (h *handler) renderNotFound(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusNotFound, "no-store", "notfound", nil)
}

// render buffers the template first so a mid-render error becomes a clean
// bodyless 500 rather than a half-written page. It reports whether the status
// was sent.
func (h *handler) render(w http.ResponseWriter, r *http.Request, status int, cacheControl, name string, data any) bool {
	var buf bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		h.logger.ErrorContext(r.Context(), "share page render failed", "template", name, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return false
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", cacheControl)
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		h.logger.ErrorContext(r.Context(), "share page write failed", "template", name, "error", err)
	}
	return true
}
