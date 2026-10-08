// Package seopage serves the programmatic SEO pages (/planning, /planning/towns,
// /planning/<authority>, /planning/<authority>/<town>) and /sitemap.xml live from
// Postgres. The routes sit behind the site build key: the public hostname is the
// Cloudflare apex Worker, which sends the key, so the API host never exposes
// duplicate content. Canonicals, JSON-LD URLs, sitemap locations and redirect
// targets always use the apex origin, never the request host.
package seopage

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/seocatalog"
)

const (
	listLimit    = 30
	cacheControl = "public, max-age=300"
	maxPathBytes = 256
)

type handler struct {
	store    Store
	buildKey string
	now      func() time.Time
	logger   *slog.Logger
}

// Routes registers GET /planning, GET /planning/{path...} and GET /sitemap.xml.
// A request without the matching X-Build-Key gets a bodyless 404. Keep the
// patterns in lockstep with anonymousPatterns in cmd/api/wiring.go.
func Routes(mux *http.ServeMux, s Store, buildKey string, now func() time.Time, logger *slog.Logger) {
	h := &handler{store: s, buildKey: buildKey, now: now, logger: logger}
	mux.HandleFunc("GET /planning", h.gated(h.servePage))
	mux.HandleFunc("GET /planning/{path...}", h.gated(h.servePage))
	mux.HandleFunc("GET /sitemap.xml", h.gated(h.serveSitemap))
}

func (h *handler) gated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !applications.BuildKeyMatches(r, h.buildKey) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		next(w, r)
	}
}

func (h *handler) servePage(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.PathValue("path"), "/")
	if len(path) > maxPathBytes {
		h.renderNotFound(w, r)
		return
	}
	ctx := r.Context()

	page, found, err := h.store.Page(ctx, path)
	if err != nil {
		h.fail(w, r, "read page", err)
		return
	}
	if !found {
		h.serveMissing(w, r, path)
		return
	}

	var (
		view    string
		data    any
		lastmod *time.Time
	)
	switch page.Kind {
	case seocatalog.KindHub:
		view = "hub"
		data, lastmod, err = h.hub(ctx)
	case seocatalog.KindTowns:
		view = "towns"
		data, lastmod, err = h.towns(ctx)
	case seocatalog.KindAuthority:
		view = "authority"
		data, lastmod, err = h.authority(ctx, page)
	case seocatalog.KindTown:
		view = "town"
		data, lastmod, err = h.town(ctx, page)
	default:
		h.renderNotFound(w, r)
		return
	}
	if err != nil {
		h.fail(w, r, "build "+view+" page", err)
		return
	}
	h.render(w, r, http.StatusOK, view, data, lastmod)
}

func (h *handler) serveMissing(w http.ResponseWriter, r *http.Request, path string) {
	target, ok, err := h.store.RedirectTarget(r.Context(), path)
	if err != nil {
		h.fail(w, r, "read redirect", err)
		return
	}
	if !ok {
		h.renderNotFound(w, r)
		return
	}
	w.Header().Set("Location", pagePath(target))
	w.Header().Set("Cache-Control", cacheControl)
	w.WriteHeader(http.StatusMovedPermanently)
}

func (h *handler) hub(ctx context.Context) (any, *time.Time, error) {
	pages, err := h.store.Pages(ctx, seocatalog.KindAuthority)
	if err != nil {
		return nil, nil, err
	}
	entries, err := h.store.Sitemap(ctx)
	if err != nil {
		return nil, nil, err
	}
	v, err := buildHubView(h.now(), pages)
	return v, newest(entries, ""), err
}

func (h *handler) towns(ctx context.Context) (any, *time.Time, error) {
	pages, err := h.store.Pages(ctx, seocatalog.KindTown)
	if err != nil {
		return nil, nil, err
	}
	entries, err := h.store.Sitemap(ctx)
	if err != nil {
		return nil, nil, err
	}
	v, err := buildTownsIndexView(h.now(), pages)
	return v, newest(entries, seocatalog.KindTown), err
}

func (h *handler) authority(ctx context.Context, page seocatalog.Page) (any, *time.Time, error) {
	apps, err := h.store.AuthorityApplications(ctx, page.AuthorityCode, listLimit)
	if err != nil {
		return nil, nil, err
	}
	lastmod, err := h.store.AuthorityLastmod(ctx, page.AuthorityCode)
	if err != nil {
		return nil, nil, err
	}
	v, err := buildAuthorityView(h.now(), page, apps, lastmod)
	return v, lastmod, err
}

func (h *handler) town(ctx context.Context, page seocatalog.Page) (any, *time.Time, error) {
	apps, err := h.store.TownApplications(ctx, page.AuthorityCode, page.TownSlug, listLimit)
	if err != nil {
		return nil, nil, err
	}
	lastmod, err := h.store.TownLastmod(ctx, page.AuthorityCode, page.TownSlug)
	if err != nil {
		return nil, nil, err
	}
	v, err := buildTownView(h.now(), page, apps, lastmod)
	return v, lastmod, err
}

// newest is the latest lastmod among entries of the given kind ("" = any kind).
func newest(entries []SitemapEntry, kind string) *time.Time {
	var max *time.Time
	for i := range entries {
		e := &entries[i]
		if e.Lastmod == nil || (kind != "" && e.Kind != kind) {
			continue
		}
		if max == nil || e.Lastmod.After(*max) {
			max = e.Lastmod
		}
	}
	return max
}

func (h *handler) serveSitemap(w http.ResponseWriter, r *http.Request) {
	entries, err := h.store.Sitemap(r.Context())
	if err != nil {
		h.fail(w, r, "read sitemap", err)
		return
	}
	for i := range entries {
		switch entries[i].Kind {
		case seocatalog.KindHub:
			entries[i].Lastmod = newest(entries, "")
		case seocatalog.KindTowns:
			entries[i].Lastmod = newest(entries, seocatalog.KindTown)
		}
	}
	body := renderSitemap(entries)
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	h.write(w, r, http.StatusOK, []byte(body), newest(entries, ""))
}

func (h *handler) renderNotFound(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusNotFound, "notfound", buildNotFoundView(h.now()), nil)
}

// render executes the named template into a buffer first so a template error
// becomes a clean bodyless 500 rather than a half-written page.
func (h *handler) render(w http.ResponseWriter, r *http.Request, status int, name string, data any, lastmod *time.Time) {
	var buf bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		h.fail(w, r, "render "+name, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.write(w, r, status, buf.Bytes(), lastmod)
}

func (h *handler) write(w http.ResponseWriter, r *http.Request, status int, body []byte, lastmod *time.Time) {
	w.Header().Set("Cache-Control", cacheControl)
	if lastmod != nil {
		w.Header().Set("Last-Modified", lastmod.UTC().Format(http.TimeFormat))
	}
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "seo page write failed", "path", r.URL.Path, "error", err)
	}
}

// fail logs a store or render error and answers a bodyless 500 (the envelope is
// backfilled by middleware). A cancelled request writes nothing: the caller is
// already gone.
func (h *handler) fail(w http.ResponseWriter, r *http.Request, op string, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	h.logger.ErrorContext(r.Context(), "seo page failed", "op", op, "path", r.URL.Path, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
