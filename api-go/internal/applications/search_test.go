package applications

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// errBoom is a fixed sentinel error for store-failure test cases.
var errBoom = errors.New("boom")

// fakeSearchStore is a hand-written searchStore fake: it records the exact
// arguments the handler passed through (query, authorityCode, limit) and
// returns a fixed result set, refine flag and error.
type fakeSearchStore struct {
	apps   []PlanningApplication
	refine bool
	err    error

	lastQuery         string
	lastAuthorityCode string
	lastLimit         int
	calls             int
}

func (f *fakeSearchStore) Search(_ context.Context, query, authorityCode string, limit int) ([]PlanningApplication, bool, error) {
	f.calls++
	f.lastQuery = query
	f.lastAuthorityCode = authorityCode
	f.lastLimit = limit
	return f.apps, f.refine, f.err
}

func serveSearch(t *testing.T, store searchStore, resolver authoritySlugResolver, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	SearchRoutes(mux, store, resolver, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// searchPath builds the search endpoint path with q query-escaped.
func searchPath(q string) string {
	return "/v1/applications/search?q=" + url.QueryEscape(q)
}

// TestSearchHandler_ValidatesQuery proves the <3-char minimum, its "looks like a
// reference" bypass (any digit), and the empty rejection, all as bodyless 400s
// that never reach the store.
func TestSearchHandler_ValidatesQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		q         string
		wantCalls int
		wantOK    bool
	}{
		{"empty", "", 0, false},
		{"whitespace only", "   ", 0, false},
		{"two chars, no digit", "ab", 0, false},
		{"three chars, no digit", "abc", 1, true},
		{"one char with digit looks like a ref", "1", 1, true},
		{"two chars with digit looks like a ref", "24", 1, true},
		{"typical short reference", "24/1", 1, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeSearchStore{}
			rec := serveSearch(t, store, testResolver(), searchPath(tc.q))

			if store.calls != tc.wantCalls {
				t.Errorf("store calls: got %d, want %d", store.calls, tc.wantCalls)
			}
			wantStatus := http.StatusBadRequest
			if tc.wantOK {
				wantStatus = http.StatusOK
			}
			if rec.Code != wantStatus {
				t.Errorf("status: got %d, want %d", rec.Code, wantStatus)
			}
		})
	}
}

// TestSearchHandler_RejectsOversizeQuery proves an excessively long q is a 400
// that never reaches the store, so a single request cannot force an expensive
// similarity/ts_rank scan with a pathological input size.
func TestSearchHandler_RejectsOversizeQuery(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("a", searchMaxQueryLen+1)
	store := &fakeSearchStore{}
	rec := serveSearch(t, store, testResolver(), searchPath(huge))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
	if store.calls != 0 {
		t.Errorf("store calls: got %d, want 0", store.calls)
	}
}

// TestSearchHandler_ReturnsMappedResults proves the response envelope shape: the
// reference field is planit_name (Name), NOT uid — echoing uid would break the
// share-URL the client builds from it (tc-geq7h.3 decision 2026-07-05).
func TestSearchHandler_ReturnsMappedResults(t *testing.T) {
	t.Parallel()

	decided := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	app := PlanningApplication{
		Name:        "24/0123/FUL",
		UID:         "24-0123-FUL-uid-distinct-from-name",
		AreaName:    "City of London",
		AreaID:      471,
		Address:     "1 Test Street",
		Description: "an extension",
		AppState:    strPtr("Permitted"),
		DecidedDate: &decided,
	}
	store := &fakeSearchStore{apps: []PlanningApplication{app}}

	rec := serveSearch(t, store, testResolver(), searchPath("extension"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}

	var got SearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Query != "extension" {
		t.Errorf("query: got %q, want %q", got.Query, "extension")
	}
	if len(got.Results) != 1 {
		t.Fatalf("results: got %d, want 1", len(got.Results))
	}
	r := got.Results[0]
	if r.Reference != app.Name {
		t.Errorf("reference: got %q, want planit_name %q (NOT uid %q)", r.Reference, app.Name, app.UID)
	}
	if r.AuthoritySlug != "city-of-london" {
		t.Errorf("authoritySlug: got %q, want %q", r.AuthoritySlug, "city-of-london")
	}
	if r.AuthorityName != app.AreaName {
		t.Errorf("authorityName: got %q, want %q", r.AuthorityName, app.AreaName)
	}
	if r.Address != app.Address {
		t.Errorf("address: got %q, want %q", r.Address, app.Address)
	}
	if r.AppState == nil || *r.AppState != "Permitted" {
		t.Errorf("appState: got %v, want Permitted", r.AppState)
	}
	if r.DecidedDate == nil || r.DecidedDate.String() != "2026-06-01" {
		t.Errorf("decidedDate: got %v, want 2026-06-01", r.DecidedDate)
	}
	if got.RefineQuery {
		t.Error("refineQuery: got true, want false")
	}
}

// TestSearchHandler_AuthorityParam proves the authority=<id-or-slug> filter
// resolves a numeric id straight through, resolves a known slug via the
// resolver, and 400s on an unresolvable slug (never reaching the store).
func TestSearchHandler_AuthorityParam(t *testing.T) {
	t.Parallel()

	t.Run("numeric id passes through unchanged", func(t *testing.T) {
		t.Parallel()
		store := &fakeSearchStore{}
		rec := serveSearch(t, store, testResolver(), searchPath("abc")+"&authority=471")
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", rec.Code)
		}
		if store.lastAuthorityCode != "471" {
			t.Errorf("authorityCode: got %q, want %q", store.lastAuthorityCode, "471")
		}
	})

	t.Run("known slug resolves to its area id", func(t *testing.T) {
		t.Parallel()
		store := &fakeSearchStore{}
		rec := serveSearch(t, store, testResolver(), searchPath("abc")+"&authority=city-of-london")
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", rec.Code)
		}
		if store.lastAuthorityCode != "471" {
			t.Errorf("authorityCode: got %q, want %q", store.lastAuthorityCode, "471")
		}
	})

	t.Run("unresolvable slug is a 400, store never called", func(t *testing.T) {
		t.Parallel()
		store := &fakeSearchStore{}
		rec := serveSearch(t, store, testResolver(), searchPath("abc")+"&authority=not-a-real-authority")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: got %d, want 400", rec.Code)
		}
		if store.calls != 0 {
			t.Errorf("store calls: got %d, want 0", store.calls)
		}
	})

	t.Run("absent authority means no filter", func(t *testing.T) {
		t.Parallel()
		store := &fakeSearchStore{}
		rec := serveSearch(t, store, testResolver(), searchPath("abc"))
		if rec.Code != http.StatusOK {
			t.Fatalf("status: got %d, want 200", rec.Code)
		}
		if store.lastAuthorityCode != "" {
			t.Errorf("authorityCode: got %q, want empty (no filter)", store.lastAuthorityCode)
		}
	})
}

// TestSearchHandler_LimitClamping proves limit defaults to 20, is capped at 20,
// and non-numeric/non-positive values fall back to the default.
func TestSearchHandler_LimitClamping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		raw       string
		wantLimit int
	}{
		{"unset defaults to 20", "", 20},
		{"within range", "5", 5},
		{"capped at 20", "9999", 20},
		{"zero falls back to default", "0", 20},
		{"negative falls back to default", "-3", 20},
		{"non-numeric falls back to default", "abc", 20},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &fakeSearchStore{}
			path := searchPath("abc")
			if tc.raw != "" {
				path += "&limit=" + tc.raw
			}
			rec := serveSearch(t, store, testResolver(), path)
			if rec.Code != http.StatusOK {
				t.Fatalf("status: got %d, want 200", rec.Code)
			}
			if store.lastLimit != tc.wantLimit {
				t.Errorf("limit: got %d, want %d", store.lastLimit, tc.wantLimit)
			}
		})
	}
}

// TestSearchHandler_RefineQueryFlag proves the store's refine signal (more
// matches exist than the capped limit) surfaces on the wire as refineQuery.
func TestSearchHandler_RefineQueryFlag(t *testing.T) {
	t.Parallel()
	store := &fakeSearchStore{refine: true}
	rec := serveSearch(t, store, testResolver(), searchPath("abc"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	var got SearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.RefineQuery {
		t.Error("refineQuery: got false, want true")
	}
}

// TestSearchHandler_ResultsNeverNull proves an empty match set marshals to []
// not null.
func TestSearchHandler_ResultsNeverNull(t *testing.T) {
	t.Parallel()
	store := &fakeSearchStore{apps: nil}
	rec := serveSearch(t, store, testResolver(), searchPath("abc"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if want := `"results":[]`; !strings.Contains(rec.Body.String(), want) {
		t.Errorf("body missing %q; body=%s", want, rec.Body.String())
	}
}

// TestSearchHandler_StoreErrorIsServerError proves a store failure is a bodyless
// 500, mirroring every other handler in this package.
func TestSearchHandler_StoreErrorIsServerError(t *testing.T) {
	t.Parallel()
	store := &fakeSearchStore{err: errBoom}
	rec := serveSearch(t, store, testResolver(), searchPath("abc"))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", rec.Code)
	}
}

// TestSearchHandler_AuthoritySlugFallback proves an application whose area id
// the resolver doesn't know falls back to slugifying its raw area name, exactly
// like the by-id/by-slug handlers.
func TestSearchHandler_AuthoritySlugFallback(t *testing.T) {
	t.Parallel()
	app := PlanningApplication{Name: "1/1", AreaName: "Some Unknown Council", AreaID: 999999}
	store := &fakeSearchStore{apps: []PlanningApplication{app}}
	rec := serveSearch(t, store, testResolver(), searchPath("abc"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var got SearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Results) != 1 {
		t.Fatalf("results: got %d, want 1", len(got.Results))
	}
	if want := "some-unknown-council"; got.Results[0].AuthoritySlug != want {
		t.Errorf("authoritySlug fallback: got %q, want %q", got.Results[0].AuthoritySlug, want)
	}
}
