package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/seocatalog"
	"github.com/AmyDe/town-crier/api-go/internal/seopage"
)

type fakeSEOStore struct{}

func (fakeSEOStore) Page(_ context.Context, path string) (seocatalog.Page, bool, error) {
	if path == "" {
		return seocatalog.Page{Kind: seocatalog.KindHub, DisplayName: "Planning applications"}, true, nil
	}
	return seocatalog.Page{}, false, nil
}

func (fakeSEOStore) RedirectTarget(context.Context, string) (string, bool, error) {
	return "", false, nil
}

func (fakeSEOStore) Pages(context.Context, string) ([]seocatalog.Page, error) { return nil, nil }

func (fakeSEOStore) AuthorityApplications(context.Context, string, int) ([]applications.PlanningApplication, error) {
	return nil, nil
}

func (fakeSEOStore) TownApplications(context.Context, string, string, int) ([]applications.PlanningApplication, error) {
	return nil, nil
}

func (fakeSEOStore) AuthorityLastmod(context.Context, string) (*time.Time, error) { return nil, nil }

func (fakeSEOStore) TownLastmod(context.Context, string, string) (*time.Time, error) { return nil, nil }

func (fakeSEOStore) Sitemap(context.Context) ([]seopage.SitemapEntry, error) { return nil, nil }

func newSEORouter(t *testing.T, buildKey string, burst int) http.Handler {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	return newRouter(denyAllValidator{}, []string{"https://towncrierapp.uk"}, nil, profiles.NoOpAuth0Client{}, profiles.CascadeDeleters{}, profiles.ExportReaders{}, nil, nil, nil, nil, fakeAppStore{}, nil, testGeocodeClient(t), testDesignationClient(t), nil, nil, "", buildKey, nil, nil, "", nil, nil, nil, burst, 60, pollingAdminDeps{}, fakeSEOStore{}, logger)
}

func seoRequest(t *testing.T, h http.Handler, path, key, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	req.RemoteAddr = remoteAddr
	if key != "" {
		req.Header.Set("X-Build-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAnonymousPatterns_IncludesSEOPages(t *testing.T) {
	t.Parallel()
	for _, pattern := range []string{"GET /planning", "GET /planning/{path...}", "GET /sitemap.xml"} {
		if _, ok := anonymousPatterns[pattern]; !ok {
			t.Errorf("anonymousPatterns must include %q", pattern)
		}
	}
}

func TestRouter_SEOPagesBuildKeyGate(t *testing.T) {
	t.Parallel()
	const buildKey = "s3cret-build-key"
	h := newSEORouter(t, buildKey, 60)

	for _, path := range []string{"/planning", "/planning/", "/planning/cornwall", "/sitemap.xml"} {
		rec := seoRequest(t, h, path, "", "203.0.113.10:1")
		if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "<") {
			t.Errorf("keyless GET %s = %d with body %q, want a 404 with no page content", path, rec.Code, rec.Body.String())
		}
	}
	if rec := seoRequest(t, h, "/planning", buildKey, "203.0.113.10:1"); rec.Code != http.StatusOK {
		t.Errorf("keyed GET /planning = %d, want 200", rec.Code)
	}
	if rec := seoRequest(t, h, "/sitemap.xml", buildKey, "203.0.113.10:1"); rec.Code != http.StatusOK {
		t.Errorf("keyed GET /sitemap.xml = %d, want 200", rec.Code)
	}
	if rec := seoRequest(t, h, "/planning/cornwall", buildKey, "203.0.113.10:1"); rec.Code != http.StatusNotFound {
		t.Errorf("keyed GET unknown page = %d, want branded 404", rec.Code)
	}
}

func TestRouter_SEOPagesExemptFromAnonRateLimitWithBuildKey(t *testing.T) {
	t.Parallel()
	const buildKey = "s3cret-build-key"
	const sameIP = "203.0.113.11:1"
	h := newSEORouter(t, buildKey, 1)

	if rec := seoRequest(t, h, "/planning", "", sameIP); rec.Code != http.StatusNotFound {
		t.Fatalf("setup: keyless request = %d, want 404 while budget remains", rec.Code)
	}
	if rec := seoRequest(t, h, "/planning", "", sameIP); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("setup: expected exhausted budget, got %d", rec.Code)
	}
	for _, path := range []string{"/planning", "/planning/towns", "/sitemap.xml"} {
		if rec := seoRequest(t, h, path, buildKey, sameIP); rec.Code == http.StatusTooManyRequests {
			t.Errorf("keyed GET %s was rate limited; build-key requests must be exempt", path)
		}
	}
}
