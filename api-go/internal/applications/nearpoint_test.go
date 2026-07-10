package applications

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeNearPointStore is a hand-written nearPointStore fake that records the
// last call's arguments and returns caller-configured results. It tracks the
// distance path (FindNearbyPage) and the recent path (RecentNearPoint, GH#912
// Phase 2) separately, so a test can assert which one the handler routed to.
type fakeNearPointStore struct {
	apps       []PlanningApplication
	nextCursor string
	err        error

	calledLat    float64
	calledLng    float64
	calledRadius float64
	calledLimit  int
	calledCursor string

	recentApps []PlanningApplication
	recentErr  error

	recentCalled       bool
	recentCalledLat    float64
	recentCalledLng    float64
	recentCalledRadius float64
	recentCalledLimit  int
}

func (f *fakeNearPointStore) FindNearbyPage(_ context.Context, latitude, longitude, radiusMetres float64, limit int, cursor string) ([]PlanningApplication, string, error) {
	f.calledLat = latitude
	f.calledLng = longitude
	f.calledRadius = radiusMetres
	f.calledLimit = limit
	f.calledCursor = cursor
	if f.err != nil {
		return nil, "", f.err
	}
	return f.apps, f.nextCursor, nil
}

func (f *fakeNearPointStore) RecentNearPoint(_ context.Context, latitude, longitude, radiusMetres float64, limit int) ([]PlanningApplication, error) {
	f.recentCalled = true
	f.recentCalledLat = latitude
	f.recentCalledLng = longitude
	f.recentCalledRadius = radiusMetres
	f.recentCalledLimit = limit
	if f.recentErr != nil {
		return nil, f.recentErr
	}
	return f.recentApps, nil
}

// newNearPointTestHandler builds a near-point mux wired to the given fake
// store and testResolver() (the shared fakeResolver from handler_test.go,
// which maps AreaID 471 <-> "city-of-london"), discarding log output.
func newNearPointTestHandler(store *fakeNearPointStore) http.Handler {
	mux := http.NewServeMux()
	NearPointRoutes(mux, store, testResolver(), slog.New(slog.DiscardHandler))
	return mux
}

// TestNearPointHandler_RequiresLatLng proves a missing or unparseable lat/lng
// is a bodyless 400 and never reaches the store.
func TestNearPointHandler_RequiresLatLng(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
	}{
		{"missing both", "/v1/applications/near-point"},
		{"missing lng", "/v1/applications/near-point?lat=51.5"},
		{"missing lat", "/v1/applications/near-point?lng=-0.1"},
		{"non-numeric lat", "/v1/applications/near-point?lat=abc&lng=-0.1"},
		{"non-numeric lng", "/v1/applications/near-point?lat=51.5&lng=xyz"},
		{"lat out of range high", "/v1/applications/near-point?lat=91&lng=-0.1"},
		{"lat out of range low", "/v1/applications/near-point?lat=-91&lng=-0.1"},
		{"lng out of range high", "/v1/applications/near-point?lat=51.5&lng=181"},
		{"lng out of range low", "/v1/applications/near-point?lat=51.5&lng=-181"},
		{"lat is NaN literal", "/v1/applications/near-point?lat=NaN&lng=-0.1"},
		{"lat is Inf literal", "/v1/applications/near-point?lat=Inf&lng=-0.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := &fakeNearPointStore{}
			h := newNearPointTestHandler(store)

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tc.url, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty (bodyless 400)", rec.Body.String())
			}
		})
	}
}

// TestNearPointHandler_ValidLatLngUsesDefaults proves a bare valid lat/lng
// call reaches the store with the documented defaults: radius 2000m, limit
// 100, no cursor.
func TestNearPointHandler_ValidLatLngUsesDefaults(t *testing.T) {
	t.Parallel()

	store := &fakeNearPointStore{}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if store.calledLat != 51.5 || store.calledLng != -0.1 {
		t.Errorf("store called with (%v, %v), want (51.5, -0.1)", store.calledLat, store.calledLng)
	}
	if store.calledRadius != nearPointDefaultRadiusMetres {
		t.Errorf("radius = %v, want default %v", store.calledRadius, nearPointDefaultRadiusMetres)
	}
	if store.calledLimit != nearPointDefaultLimit {
		t.Errorf("limit = %v, want default %v", store.calledLimit, nearPointDefaultLimit)
	}
	if store.calledCursor != "" {
		t.Errorf("cursor = %q, want empty first page", store.calledCursor)
	}
}

// TestNearPointHandler_ResponseShape proves the body is a bare JSON array of
// NearbyResult (not wrapped in an envelope), matching the authed nearby page.
func TestNearPointHandler_ResponseShape(t *testing.T) {
	t.Parallel()

	app := PlanningApplication{Name: "24/0001/FUL", UID: "uid-1", AreaName: "Testshire", AreaID: 100, Address: "1 Test Street"}
	store := &fakeNearPointStore{apps: []PlanningApplication{app}}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}

	var results []NearbyResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("body is not a bare JSON array: %v; body = %s", err, rec.Body.String())
	}
	if len(results) != 1 || results[0].Name != app.Name || results[0].UID != app.UID {
		t.Errorf("results = %+v, want one NearbyResult matching %+v", results, app)
	}
}

// TestNearPointHandler_AuthoritySlugResolved proves each result carries its
// authority's resolved URL slug (GH#879 Phase 1) — the field ResultOf leaves
// unset on the authed embeddings (watchzones/savedapplications), but which the
// anonymous near-point endpoint needs so a client can build a share URL or a
// by-slug detail fetch without ever holding a session.
func TestNearPointHandler_AuthoritySlugResolved(t *testing.T) {
	t.Parallel()

	app := PlanningApplication{Name: "1/1", UID: "uid-471", AreaName: "City of London", AreaID: 471}
	store := &fakeNearPointStore{apps: []PlanningApplication{app}}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var results []NearbyResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results: got %d, want 1", len(results))
	}
	if want := "city-of-london"; results[0].AuthoritySlug != want {
		t.Errorf("authoritySlug = %q, want %q", results[0].AuthoritySlug, want)
	}
}

// TestNearPointHandler_AuthoritySlugFallback proves an application whose area
// id the resolver doesn't know falls back to slugifying its raw area name,
// exactly like the search and by-id/by-slug handlers (see
// TestSearchHandler_AuthoritySlugFallback).
func TestNearPointHandler_AuthoritySlugFallback(t *testing.T) {
	t.Parallel()

	app := PlanningApplication{Name: "1/1", UID: "uid-999999", AreaName: "Some Unknown Council", AreaID: 999999}
	store := &fakeNearPointStore{apps: []PlanningApplication{app}}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var results []NearbyResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results: got %d, want 1", len(results))
	}
	if want := "some-unknown-council"; results[0].AuthoritySlug != want {
		t.Errorf("authoritySlug fallback: got %q, want %q", results[0].AuthoritySlug, want)
	}
}

// TestNearPointHandler_RadiusClamping proves ?radius= is clamped into
// [100, 5000], defaulting to 2000 when unset or unparseable, and passing a
// genuinely in-range value straight through.
func TestNearPointHandler_RadiusClamping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		radius     string
		wantRadius float64
	}{
		{"unset falls back to default", "", nearPointDefaultRadiusMetres},
		{"non-numeric falls back to default", "banana", nearPointDefaultRadiusMetres},
		{"below minimum clamps to minimum", "50", nearPointMinRadiusMetres},
		{"zero clamps to minimum", "0", nearPointMinRadiusMetres},
		{"negative clamps to minimum", "-500", nearPointMinRadiusMetres},
		{"above maximum clamps to maximum", "9000", nearPointMaxRadiusMetres},
		{"in range passes through", "1500", 1500},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := &fakeNearPointStore{}
			h := newNearPointTestHandler(store)

			url := "/v1/applications/near-point?lat=51.5&lng=-0.1"
			if tc.name != "unset falls back to default" {
				url += "&radius=" + tc.radius
			}

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
			}
			if store.calledRadius != tc.wantRadius {
				t.Errorf("radius = %v, want %v", store.calledRadius, tc.wantRadius)
			}
		})
	}
}

// TestNearPointHandler_LimitClamping proves ?limit= is clamped into
// [1, 200], defaulting to 100 when unset or unparseable, and passing a
// genuinely in-range value straight through.
func TestNearPointHandler_LimitClamping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		limit     string
		wantLimit int
	}{
		{"unset falls back to default", "", nearPointDefaultLimit},
		{"non-numeric falls back to default", "banana", nearPointDefaultLimit},
		{"zero clamps to one", "0", 1},
		{"negative clamps to one", "-5", 1},
		{"above maximum clamps to maximum", "500", nearPointMaxLimit},
		{"in range passes through", "42", 42},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := &fakeNearPointStore{}
			h := newNearPointTestHandler(store)

			url := "/v1/applications/near-point?lat=51.5&lng=-0.1"
			if tc.name != "unset falls back to default" {
				url += "&limit=" + tc.limit
			}

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
			}
			if store.calledLimit != tc.wantLimit {
				t.Errorf("limit = %v, want %v", store.calledLimit, tc.wantLimit)
			}
		})
	}
}

// TestNearPointHandler_CursorRoundTrip proves the store's next-page token
// round-trips through the X-Next-Cursor response header and back through
// ?cursor= to the exact same store call.
func TestNearPointHandler_CursorRoundTrip(t *testing.T) {
	t.Parallel()

	store := &fakeNearPointStore{nextCursor: "raw-keyset-token"}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	nextHeader := rec.Header().Get("X-Next-Cursor")
	if nextHeader == "" {
		t.Fatal("expected X-Next-Cursor header on a full page")
	}
	if nextHeader == store.nextCursor {
		t.Error("X-Next-Cursor should be base64url-wrapped, not the raw store token")
	}

	store2 := &fakeNearPointStore{}
	h2 := newNearPointTestHandler(store2)
	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1&cursor="+nextHeader, nil)
	rec2 := httptest.NewRecorder()
	h2.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec2.Code, rec2.Body.String())
	}
	if store2.calledCursor != store.nextCursor {
		t.Errorf("decoded cursor = %q, want %q", store2.calledCursor, store.nextCursor)
	}
}

// TestNearPointHandler_CursorOmittedOnLastPage proves the header is absent
// when the store reports no further page.
func TestNearPointHandler_CursorOmittedOnLastPage(t *testing.T) {
	t.Parallel()

	store := &fakeNearPointStore{}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("X-Next-Cursor"); got != "" {
		t.Errorf("X-Next-Cursor = %q, want absent on last page", got)
	}
}

// TestNearPointHandler_MalformedCursorIsBadRequest proves a malformed cursor
// is rejected as a clean 400, never silently reset to page one.
func TestNearPointHandler_MalformedCursorIsBadRequest(t *testing.T) {
	t.Parallel()

	store := &fakeNearPointStore{}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1&cursor=not-valid-base64url!!!", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty (bodyless 400)", rec.Body.String())
	}
}

// TestNearPointHandler_StoreError proves a store failure is a bodyless 500.
func TestNearPointHandler_StoreError(t *testing.T) {
	t.Parallel()

	store := &fakeNearPointStore{err: errors.New("boom")}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty (bodyless 500)", rec.Body.String())
	}
}

// TestParseNearPointSort proves the ?sort= vocabulary: omitted/"distance" is the
// default (ok, normalised to nearPointSortDistance), "recent" is accepted, and
// anything else is rejected (ok == false) so the handler can return a clean 400
// — mirroring parseNearPointCoordinates/decodeNearPointCursor's reject-don't-guess
// convention rather than the radius/limit clamp-don't-reject convention (GH#912
// Phase 2: there is no sensible "nearest legal sort" to clamp an unknown value to).
func TestParseNearPointSort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		wantSort string
		wantOK   bool
	}{
		{"empty defaults to distance", "", nearPointSortDistance, true},
		{"explicit distance", "distance", nearPointSortDistance, true},
		{"recent", "recent", nearPointSortRecent, true},
		{"unknown value rejected", "banana", "", false},
		{"case sensitive: uppercase rejected", "Recent", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := parseNearPointSort(tc.raw)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got != tc.wantSort {
				t.Errorf("sort = %q, want %q", got, tc.wantSort)
			}
		})
	}
}

// TestNearPointHandler_InvalidSortIsBadRequest proves an unrecognised ?sort=
// value is a bodyless 400 and never reaches either store method (GH#912 Phase 2).
func TestNearPointHandler_InvalidSortIsBadRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sort string
	}{
		{"unknown word", "banana"},
		{"wrong case", "Recent"},
		{"legacy activity sort not accepted here", "recent-activity"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := &fakeNearPointStore{}
			h := newNearPointTestHandler(store)

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
				"/v1/applications/near-point?lat=51.5&lng=-0.1&sort="+tc.sort, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if rec.Body.Len() != 0 {
				t.Errorf("body = %q, want empty (bodyless 400)", rec.Body.String())
			}
			if store.calledLimit != 0 || store.recentCalled {
				t.Error("an invalid sort must never reach either store method")
			}
		})
	}
}

// TestNearPointHandler_SortDistanceIsDefaultAndByteIdentical proves an omitted
// ?sort= and an explicit ?sort=distance both take the legacy FindNearbyPage path
// and produce byte-identical response bodies — the acceptance requirement that
// the API change is fully backward compatible for every existing caller (GH#912
// Phase 2).
func TestNearPointHandler_SortDistanceIsDefaultAndByteIdentical(t *testing.T) {
	t.Parallel()

	app := PlanningApplication{Name: "24/0001/FUL", UID: "uid-1", AreaName: "Testshire", AreaID: 100}

	omittedStore := &fakeNearPointStore{apps: []PlanningApplication{app}}
	hOmitted := newNearPointTestHandler(omittedStore)
	reqOmitted := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1", nil)
	recOmitted := httptest.NewRecorder()
	hOmitted.ServeHTTP(recOmitted, reqOmitted)

	explicitStore := &fakeNearPointStore{apps: []PlanningApplication{app}}
	hExplicit := newNearPointTestHandler(explicitStore)
	reqExplicit := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1&sort=distance", nil)
	recExplicit := httptest.NewRecorder()
	hExplicit.ServeHTTP(recExplicit, reqExplicit)

	if recOmitted.Code != http.StatusOK || recExplicit.Code != http.StatusOK {
		t.Fatalf("status = %d/%d, want 200/200", recOmitted.Code, recExplicit.Code)
	}
	if recOmitted.Body.String() != recExplicit.Body.String() {
		t.Errorf("body mismatch: omitted = %q, explicit = %q", recOmitted.Body.String(), recExplicit.Body.String())
	}
	if omittedStore.recentCalled || explicitStore.recentCalled {
		t.Error("distance sort (default or explicit) must never call RecentNearPoint")
	}
}

// TestNearPointHandler_SortRecentCallsRecentNearPoint proves ?sort=recent routes
// to the store's RecentNearPoint (not FindNearbyPage), passes lat/lng through
// unchanged, returns the recent-path results as the bare JSON array, and never
// sets X-Next-Cursor (RecentNearPoint is a single bounded page, mirroring
// RecentByAuthority/RecentNearestTown — see RecentNearPoint's doc comment).
func TestNearPointHandler_SortRecentCallsRecentNearPoint(t *testing.T) {
	t.Parallel()

	app := PlanningApplication{Name: "24/0002/FUL", UID: "uid-2", AreaName: "Testshire", AreaID: 100}
	store := &fakeNearPointStore{
		apps:       []PlanningApplication{{Name: "wrong-path", UID: "should-not-appear"}},
		recentApps: []PlanningApplication{app},
	}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1&sort=recent", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if !store.recentCalled {
		t.Fatal("expected RecentNearPoint to be called")
	}
	if store.calledLimit != 0 {
		t.Error("sort=recent must never call the legacy FindNearbyPage path")
	}
	if store.recentCalledLat != 51.5 || store.recentCalledLng != -0.1 {
		t.Errorf("RecentNearPoint called with (%v, %v), want (51.5, -0.1)", store.recentCalledLat, store.recentCalledLng)
	}

	var results []NearbyResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("body is not a bare JSON array: %v; body = %s", err, rec.Body.String())
	}
	if len(results) != 1 || results[0].Name != app.Name {
		t.Errorf("results = %+v, want one NearbyResult matching %+v", results, app)
	}
	if got := rec.Header().Get("X-Next-Cursor"); got != "" {
		t.Errorf("X-Next-Cursor = %q, want absent for sort=recent", got)
	}
}

// TestNearPointHandler_SortRecentAppliesRadiusAndLimitClamp proves the radius
// and limit clamps still apply on the recent path exactly as on the distance
// path (GH#912 Phase 2: "radius clamp [100,5000]/default 2000 and limit clamp
// [1,200]/default 100 unchanged").
func TestNearPointHandler_SortRecentAppliesRadiusAndLimitClamp(t *testing.T) {
	t.Parallel()

	store := &fakeNearPointStore{}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet,
		"/v1/applications/near-point?lat=51.5&lng=-0.1&sort=recent&radius=9000&limit=500", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if store.recentCalledRadius != nearPointMaxRadiusMetres {
		t.Errorf("radius = %v, want clamped max %v", store.recentCalledRadius, nearPointMaxRadiusMetres)
	}
	if store.recentCalledLimit != nearPointMaxLimit {
		t.Errorf("limit = %v, want clamped max %v", store.recentCalledLimit, nearPointMaxLimit)
	}
}

// TestNearPointHandler_SortRecentStoreError proves a RecentNearPoint failure is
// a bodyless 500, mirroring TestNearPointHandler_StoreError for the distance path.
func TestNearPointHandler_SortRecentStoreError(t *testing.T) {
	t.Parallel()

	store := &fakeNearPointStore{recentErr: errors.New("boom")}
	h := newNearPointTestHandler(store)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/applications/near-point?lat=51.5&lng=-0.1&sort=recent", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty (bodyless 500)", rec.Body.String())
	}
}
