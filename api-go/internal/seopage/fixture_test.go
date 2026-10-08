package seopage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/seocatalog"
	"log/slog"
)

const testBuildKey = "test-build-key"

var fixedNow = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

type fixtureApp struct {
	UID           string  `json:"uid"`
	Name          string  `json:"name"`
	Address       string  `json:"address"`
	Description   string  `json:"description"`
	AppState      *string `json:"appState"`
	StartDate     *string `json:"startDate"`
	DecidedDate   *string `json:"decidedDate"`
	LastDifferent string  `json:"lastDifferent"`
	Link          *string `json:"link"`
	URL           *string `json:"url"`
}

type fixturePage struct {
	Path            string                  `json:"path"`
	Kind            string                  `json:"kind"`
	AuthorityCode   string                  `json:"authorityCode"`
	TownSlug        string                  `json:"townSlug"`
	DisplayName     string                  `json:"displayName"`
	AuthorityName   string                  `json:"authorityName"`
	Total           int                     `json:"total"`
	StatusBreakdown []seocatalog.StateCount `json:"statusBreakdown"`
	Children        []seocatalog.TownLink   `json:"children"`
	Neighbours      []seocatalog.Neighbour  `json:"neighbours"`
	SortOrder       int                     `json:"sortOrder"`
}

type fixtureFile struct {
	Pages        []fixturePage           `json:"pages"`
	Redirects    []seocatalog.Redirect   `json:"redirects"`
	Lastmod      map[string]string       `json:"lastmod"`
	Applications map[string][]fixtureApp `json:"applications"`
}

func parseDate(t *testing.T, s *string) *time.Time {
	t.Helper()
	if s == nil {
		return nil
	}
	d, err := time.Parse("2006-01-02", *s)
	if err != nil {
		t.Fatalf("parse date %q: %v", *s, err)
	}
	return &d
}

type fakeStore struct {
	pages     []seocatalog.Page
	redirects map[string]string
	apps      map[string][]applications.PlanningApplication
	lastmod   map[string]*time.Time
	err       error
}

func loadFixture(t *testing.T) *fakeStore {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "catalog.json"))
	if err != nil {
		t.Fatalf("read catalog fixture: %v", err)
	}
	var f fixtureFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode catalog fixture: %v", err)
	}
	s := &fakeStore{redirects: map[string]string{}, apps: map[string][]applications.PlanningApplication{}, lastmod: map[string]*time.Time{}}
	for _, p := range f.Pages {
		s.pages = append(s.pages, seocatalog.Page{
			Path: p.Path, Kind: p.Kind, AuthorityCode: p.AuthorityCode, TownSlug: p.TownSlug, DisplayName: p.DisplayName,
			AuthorityName: p.AuthorityName, Total: p.Total, StatusBreakdown: p.StatusBreakdown, Children: p.Children,
			Neighbours: p.Neighbours, SortOrder: p.SortOrder,
		})
	}
	for _, r := range f.Redirects {
		s.redirects[r.Path] = r.Target
	}
	for path, day := range f.Lastmod {
		s.lastmod[path] = parseDate(t, &day)
	}
	for path, list := range f.Applications {
		for _, a := range list {
			ld, err := time.Parse(time.RFC3339, a.LastDifferent)
			if err != nil {
				t.Fatalf("parse lastDifferent %q: %v", a.LastDifferent, err)
			}
			s.apps[path] = append(s.apps[path], applications.PlanningApplication{
				UID: a.UID, Name: a.Name, Address: a.Address, Description: a.Description, AppState: a.AppState,
				StartDate: parseDate(t, a.StartDate), DecidedDate: parseDate(t, a.DecidedDate), LastDifferent: ld,
				Link: a.Link, URL: a.URL,
			})
		}
	}
	return s
}

func (s *fakeStore) byCode(kind, code, slug string) (seocatalog.Page, bool) {
	for _, p := range s.pages {
		if p.Kind == kind && p.AuthorityCode == code && p.TownSlug == slug {
			return p, true
		}
	}
	return seocatalog.Page{}, false
}

func (s *fakeStore) Page(_ context.Context, path string) (seocatalog.Page, bool, error) {
	if s.err != nil {
		return seocatalog.Page{}, false, s.err
	}
	for _, p := range s.pages {
		if p.Path == path {
			return p, true, nil
		}
	}
	return seocatalog.Page{}, false, nil
}

func (s *fakeStore) RedirectTarget(_ context.Context, path string) (string, bool, error) {
	target, ok := s.redirects[path]
	return target, ok, nil
}

func (s *fakeStore) Pages(_ context.Context, kind string) ([]seocatalog.Page, error) {
	var out []seocatalog.Page
	for _, p := range s.pages {
		if p.Kind == kind {
			out = append(out, p)
		}
	}
	return out, nil
}

func (s *fakeStore) AuthorityApplications(_ context.Context, code string, limit int) ([]applications.PlanningApplication, error) {
	p, ok := s.byCode(seocatalog.KindAuthority, code, "")
	if !ok {
		return nil, nil
	}
	return s.apps[p.Path][:min(limit, len(s.apps[p.Path]))], nil
}

func (s *fakeStore) TownApplications(_ context.Context, code, slug string, limit int) ([]applications.PlanningApplication, error) {
	p, ok := s.byCode(seocatalog.KindTown, code, slug)
	if !ok {
		return nil, nil
	}
	return s.apps[p.Path][:min(limit, len(s.apps[p.Path]))], nil
}

func (s *fakeStore) AuthorityLastmod(_ context.Context, code string) (*time.Time, error) {
	p, _ := s.byCode(seocatalog.KindAuthority, code, "")
	return s.lastmod[p.Path], nil
}

func (s *fakeStore) TownLastmod(_ context.Context, code, slug string) (*time.Time, error) {
	p, _ := s.byCode(seocatalog.KindTown, code, slug)
	return s.lastmod[p.Path], nil
}

func (s *fakeStore) Sitemap(context.Context) ([]SitemapEntry, error) {
	if s.err != nil {
		return nil, s.err
	}
	out := make([]SitemapEntry, 0, len(s.pages))
	for _, p := range s.pages {
		out = append(out, SitemapEntry{Path: p.Path, Kind: p.Kind, Lastmod: s.lastmod[p.Path]})
	}
	return out, nil
}

func newTestMux(t *testing.T, s Store) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, s, testBuildKey, func() time.Time { return fixedNow }, slog.New(slog.DiscardHandler))
	return mux
}

type response struct {
	Code   int
	Header http.Header
	Body   string
}

func get(t *testing.T, h http.Handler, path string, withKey bool) response {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	if withKey {
		req.Header.Set("X-Build-Key", testBuildKey)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return response{Code: rec.Code, Header: rec.Header(), Body: rec.Body.String()}
}

var errStore = errors.New("store unavailable")

func newKeyedRequest(path, key string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	req.Header.Set("X-Build-Key", key)
	return req
}

func recordTo(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
