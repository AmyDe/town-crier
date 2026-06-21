package applications

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeNearStore is a hand-written double for the recent-nearby read. It records
// the arguments the handler passed (so clamping/defaulting is asserted at the
// store boundary) and honours cap the way Cosmos TOP @cap does.
type fakeNearStore struct {
	apps []PlanningApplication
	err  error

	called            bool
	lastAuthorityCode string
	lastLat           float64
	lastLng           float64
	lastRadius        float64
	lastCap           int
}

func (f *fakeNearStore) RecentNearby(_ context.Context, authorityCode string, lat, lng, radiusMetres float64, cap int) ([]PlanningApplication, error) {
	f.called = true
	f.lastAuthorityCode = authorityCode
	f.lastLat = lat
	f.lastLng = lng
	f.lastRadius = radiusMetres
	f.lastCap = cap
	if f.err != nil {
		return nil, f.err
	}
	if cap >= 0 && cap < len(f.apps) {
		return f.apps[:cap], nil
	}
	return f.apps, nil
}

func serveNear(t *testing.T, store nearStore, buildKey, providedKey, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NearRoutes(mux, store, buildKey, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	if providedKey != "" {
		req.Header.Set("X-Build-Key", providedKey)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestNearHandler_Returns200WithValidKeyAndNonNullArray(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5155&lng=-0.0931&radius=4000")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}
	// The handler scopes to the numeric authority id -> partition code, reads the
	// bounded cap (200), and passes the parsed point + radius through verbatim.
	if store.lastAuthorityCode != "471" || store.lastCap != 200 {
		t.Errorf("store call: authorityCode=%q cap=%d, want \"471\" 200", store.lastAuthorityCode, store.lastCap)
	}
	if store.lastLat != 51.5155 || store.lastLng != -0.0931 || store.lastRadius != 4000 {
		t.Errorf("store geo args: lat=%v lng=%v radius=%v, want 51.5155 -0.0931 4000", store.lastLat, store.lastLng, store.lastRadius)
	}
	got := recentBody(t, rec)
	if got["authorityId"].(float64) != 471 {
		t.Errorf("authorityId: got %v, want 471", got["authorityId"])
	}
	if got["lat"].(float64) != 51.5155 || got["lng"].(float64) != -0.0931 || got["radius"].(float64) != 4000 {
		t.Errorf("echoed geo: lat=%v lng=%v radius=%v", got["lat"], got["lng"], got["radius"])
	}
	apps, ok := got["applications"].([]any)
	if !ok {
		t.Fatalf("applications must be a non-null array, got %T (%v)", got["applications"], got["applications"])
	}
	if len(apps) != 2 {
		t.Errorf("applications length: got %d, want 2", len(apps))
	}
	if got["total"].(float64) != 2 {
		t.Errorf("total: got %v, want 2", got["total"])
	}
	if got["totalCapped"].(bool) {
		t.Errorf("totalCapped: got true, want false")
	}
	first := apps[0].(map[string]any)
	for _, key := range []string{"uid", "name", "address", "description", "appState", "startDate", "link", "url"} {
		if _, present := first[key]; !present {
			t.Errorf("application missing field %q: %v", key, first)
		}
	}
}

func TestNearHandler_RejectsMissingKey(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("401 must be bodyless (backfilled downstream), got %s", rec.Body)
	}
	if store.called {
		t.Errorf("store must not be hit when the key is missing")
	}
}

func TestNearHandler_EmptyConfiguredKeyRejectsAll(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "", "anything",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("empty configured key must reject all: got %d, want 401", rec.Code)
	}
	if store.called {
		t.Errorf("store must not be hit when the configured key is empty")
	}
}

func TestNearHandler_MissingAuthorityIdReturns400(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?lat=51.5&lng=-0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing authorityId: got %d, want 400", rec.Code)
	}
	if store.called {
		t.Errorf("store must not be hit without a valid authorityId")
	}
}

func TestNearHandler_NonIntAuthorityIdReturns400(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=abc&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-int authorityId: got %d, want 400", rec.Code)
	}
	if store.called {
		t.Errorf("store must not be hit on a malformed authorityId")
	}
}

func TestNearHandler_RejectsNonFiniteLat(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=NaN&lng=-0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("NaN lat: got %d, want 400", rec.Code)
	}
	if store.called {
		t.Errorf("store must not be hit on a non-finite lat")
	}
}

func TestNearHandler_RejectsNonFiniteLng(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=Inf")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Inf lng: got %d, want 400", rec.Code)
	}
	if store.called {
		t.Errorf("store must not be hit on a non-finite lng")
	}
}

func TestNearHandler_RejectsOutOfRangeLat(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=91&lng=-0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range lat: got %d, want 400", rec.Code)
	}
	if store.called {
		t.Errorf("store must not be hit on an out-of-range lat")
	}
}

func TestNearHandler_RejectsOutOfRangeLng(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=200")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range lng: got %d, want 400", rec.Code)
	}
	if store.called {
		t.Errorf("store must not be hit on an out-of-range lng")
	}
}

func TestNearHandler_RejectsMissingLat(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lng=-0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing lat: got %d, want 400", rec.Code)
	}
	if store.called {
		t.Errorf("store must not be hit without a lat")
	}
}

func TestNearHandler_RejectsNonFiniteRadius(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1&radius=Inf")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-finite radius: got %d, want 400", rec.Code)
	}
	if store.called {
		t.Errorf("store must not be hit on a non-finite radius")
	}
}

func TestNearHandler_RejectsNonPositiveRadius(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1&radius=0")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-positive radius: got %d, want 400", rec.Code)
	}
	if store.called {
		t.Errorf("store must not be hit on a non-positive radius")
	}
}

func TestNearHandler_ClampsRadiusToMax(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1&radius=99999")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	// The clamped 10km radius is what reaches the store and what the response echoes.
	if store.lastRadius != 10000 {
		t.Errorf("store radius: got %v, want 10000 (clamped to 10km)", store.lastRadius)
	}
	got := recentBody(t, rec)
	if got["radius"].(float64) != 10000 {
		t.Errorf("echoed radius: got %v, want 10000", got["radius"])
	}
}

func TestNearHandler_DefaultRadiusWhenUnset(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if store.lastRadius != 5000 {
		t.Errorf("store radius: got %v, want 5000 (default when unset)", store.lastRadius)
	}
	got := recentBody(t, rec)
	if got["radius"].(float64) != 5000 {
		t.Errorf("echoed radius: got %v, want 5000", got["radius"])
	}
}

func TestNearHandler_DefaultLimitIs30(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 50)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	got := recentBody(t, rec)
	apps := got["applications"].([]any)
	if len(apps) != 30 {
		t.Errorf("applications length: got %d, want 30 (default limit)", len(apps))
	}
	if got["total"].(float64) != 50 {
		t.Errorf("total: got %v, want 50 (full bounded read)", got["total"])
	}
}

func TestNearHandler_LimitHardCappedAt100(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 150)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1&limit=999")

	got := recentBody(t, rec)
	apps := got["applications"].([]any)
	if len(apps) != 100 {
		t.Errorf("applications length: got %d, want 100 (hard max limit)", len(apps))
	}
}

func TestNearHandler_TotalCappedWhenReadHitsCap(t *testing.T) {
	t.Parallel()
	// Exactly cap (200) documents available -> the bounded read returns cap.
	store := &fakeNearStore{apps: recentApps(t, 200)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	got := recentBody(t, rec)
	if got["total"].(float64) != 200 {
		t.Errorf("total: got %v, want 200", got["total"])
	}
	if !got["totalCapped"].(bool) {
		t.Errorf("totalCapped: got false, want true when the read hits cap")
	}
	if apps := got["applications"].([]any); len(apps) != 30 {
		t.Errorf("applications length: got %d, want 30", len(apps))
	}
}

func TestNearHandler_EmptyResultIsNonNullArray(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: nil}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	got := recentBody(t, rec)
	apps, ok := got["applications"].([]any)
	if !ok {
		t.Fatalf("applications must be a non-null array even when empty, got %T", got["applications"])
	}
	if len(apps) != 0 {
		t.Errorf("applications length: got %d, want 0", len(apps))
	}
	if got["total"].(float64) != 0 {
		t.Errorf("total: got %v, want 0", got["total"])
	}
}

func TestNearHandler_StoreErrorReturns500(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{err: context.DeadlineExceeded}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("store error: got %d, want 500", rec.Code)
	}
}
