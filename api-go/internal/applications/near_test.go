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
// store boundary), honours cap the way Cosmos TOP @cap does, and serves an exact
// in-radius count independently of the bounded read.
type fakeNearStore struct {
	apps     []PlanningApplication
	err      error
	count    int
	countErr error

	called            bool
	lastAuthorityCode string
	lastLat           float64
	lastLng           float64
	lastRadius        float64
	lastCap           int

	// nearestCalled records whether the distance-ordered read path was taken
	// (order=distance) instead of the recency-ordered RecentNearby default.
	nearestCalled bool

	countCalled       bool
	lastCountAuthCode string
	lastCountLat      float64
	lastCountLng      float64
	lastCountRadius   float64
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

func (f *fakeNearStore) NearestNearby(_ context.Context, authorityCode string, lat, lng, radiusMetres float64, cap int) ([]PlanningApplication, error) {
	f.called = true
	f.nearestCalled = true
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

func (f *fakeNearStore) CountNearby(_ context.Context, authorityCode string, lat, lng, radiusMetres float64) (int, error) {
	f.countCalled = true
	f.lastCountAuthCode = authorityCode
	f.lastCountLat = lat
	f.lastCountLng = lng
	f.lastCountRadius = radiusMetres
	if f.countErr != nil {
		return 0, f.countErr
	}
	return f.count, nil
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
	store := &fakeNearStore{apps: recentApps(t, 2), count: 1234}

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
		t.Errorf("store read call: authorityCode=%q cap=%d, want \"471\" 200", store.lastAuthorityCode, store.lastCap)
	}
	if store.lastLat != 51.5155 || store.lastLng != -0.0931 || store.lastRadius != 4000 {
		t.Errorf("store geo args: lat=%v lng=%v radius=%v, want 51.5155 -0.0931 4000", store.lastLat, store.lastLng, store.lastRadius)
	}
	// The exact count is taken over the same scoped, clamped geo window.
	if !store.countCalled || store.lastCountAuthCode != "471" ||
		store.lastCountLat != 51.5155 || store.lastCountLng != -0.0931 || store.lastCountRadius != 4000 {
		t.Errorf("count call: called=%v auth=%q lat=%v lng=%v radius=%v, want true \"471\" 51.5155 -0.0931 4000",
			store.countCalled, store.lastCountAuthCode, store.lastCountLat, store.lastCountLng, store.lastCountRadius)
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
	if got["total"].(float64) != 1234 {
		t.Errorf("total: got %v, want 1234 (the exact count)", got["total"])
	}
	if _, present := got["totalCapped"]; present {
		t.Errorf("totalCapped must be gone from the wire shape, got %v", got["totalCapped"])
	}
	if _, present := got["statusBreakdown"]; !present {
		t.Errorf("statusBreakdown must be present: %v", got)
	}
	first := apps[0].(map[string]any)
	for _, key := range []string{"uid", "name", "address", "description", "appState", "startDate", "lastDifferent", "link", "url"} {
		if _, present := first[key]; !present {
			t.Errorf("application missing field %q: %v", key, first)
		}
	}
}

func TestNearHandler_RejectsMissingKey(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 2}

	rec := serveNear(t, store, "buildkey", "",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("401 must be bodyless (backfilled downstream), got %s", rec.Body)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit when the key is missing")
	}
}

func TestNearHandler_EmptyConfiguredKeyRejectsAll(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 2}

	rec := serveNear(t, store, "", "anything",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("empty configured key must reject all: got %d, want 401", rec.Code)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit when the configured key is empty")
	}
}

func TestNearHandler_MissingAuthorityIdReturns400(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 2}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?lat=51.5&lng=-0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing authorityId: got %d, want 400", rec.Code)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit without a valid authorityId")
	}
}

func TestNearHandler_NonIntAuthorityIdReturns400(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 2}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=abc&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-int authorityId: got %d, want 400", rec.Code)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit on a malformed authorityId")
	}
}

func TestNearHandler_RejectsNonFiniteLat(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 2}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=NaN&lng=-0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("NaN lat: got %d, want 400", rec.Code)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit on a non-finite lat")
	}
}

func TestNearHandler_RejectsNonFiniteLng(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 2}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=Inf")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Inf lng: got %d, want 400", rec.Code)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit on a non-finite lng")
	}
}

func TestNearHandler_RejectsOutOfRangeLat(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 2}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=91&lng=-0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range lat: got %d, want 400", rec.Code)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit on an out-of-range lat")
	}
}

func TestNearHandler_RejectsOutOfRangeLng(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 2}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=200")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range lng: got %d, want 400", rec.Code)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit on an out-of-range lng")
	}
}

func TestNearHandler_RejectsMissingLat(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 2}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lng=-0.1")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing lat: got %d, want 400", rec.Code)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit without a lat")
	}
}

func TestNearHandler_RejectsNonFiniteRadius(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 2}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1&radius=Inf")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-finite radius: got %d, want 400", rec.Code)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit on a non-finite radius")
	}
}

func TestNearHandler_RejectsNonPositiveRadius(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 2}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1&radius=0")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-positive radius: got %d, want 400", rec.Code)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit on a non-positive radius")
	}
}

func TestNearHandler_ClampsRadiusToMax(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 2}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1&radius=99999")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	// The clamped 10km radius is what reaches both the read and the count, and what
	// the response echoes.
	if store.lastRadius != 10000 || store.lastCountRadius != 10000 {
		t.Errorf("store radius: read=%v count=%v, want 10000 (clamped to 10km)", store.lastRadius, store.lastCountRadius)
	}
	got := recentBody(t, rec)
	if got["radius"].(float64) != 10000 {
		t.Errorf("echoed radius: got %v, want 10000", got["radius"])
	}
}

func TestNearHandler_DefaultRadiusWhenUnset(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 2}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if store.lastRadius != 5000 || store.lastCountRadius != 5000 {
		t.Errorf("store radius: read=%v count=%v, want 5000 (default when unset)", store.lastRadius, store.lastCountRadius)
	}
	got := recentBody(t, rec)
	if got["radius"].(float64) != 5000 {
		t.Errorf("echoed radius: got %v, want 5000", got["radius"])
	}
}

func TestNearHandler_DefaultLimitIs30(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 50), count: 73}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	got := recentBody(t, rec)
	apps := got["applications"].([]any)
	if len(apps) != 30 {
		t.Errorf("applications length: got %d, want 30 (default limit)", len(apps))
	}
	if got["total"].(float64) != 73 {
		t.Errorf("total: got %v, want 73 (exact count)", got["total"])
	}
}

func TestNearHandler_LimitHardCappedAt100(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 150), count: 150}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1&limit=999")

	got := recentBody(t, rec)
	apps := got["applications"].([]any)
	if len(apps) != 100 {
		t.Errorf("applications length: got %d, want 100 (hard max limit)", len(apps))
	}
}

func TestNearHandler_ExactTotalIndependentOfSaturatedRead(t *testing.T) {
	t.Parallel()
	// The bounded read saturates at cap (200) but the exact count is far larger:
	// total must be the count, and the render slice clamps to the limit.
	store := &fakeNearStore{apps: recentApps(t, 200), count: 6502}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	got := recentBody(t, rec)
	if got["total"].(float64) != 6502 {
		t.Errorf("total: got %v, want 6502 (exact count)", got["total"])
	}
	if apps := got["applications"].([]any); len(apps) != 30 {
		t.Errorf("applications length: got %d, want 30 (default limit, NOT the count)", len(apps))
	}
}

func TestNearHandler_StatusBreakdownSpansBoundedReadNotRenderedSlice(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: appsByState(t, 25, 15), count: 40}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	got := recentBody(t, rec)
	breakdown, ok := got["statusBreakdown"].([]any)
	if !ok {
		t.Fatalf("statusBreakdown must be an array, got %T", got["statusBreakdown"])
	}
	if len(breakdown) != 2 {
		t.Fatalf("statusBreakdown length: got %d, want 2 (%v)", len(breakdown), breakdown)
	}
	first := breakdown[0].(map[string]any)
	if first["appState"] != "Permitted" || first["count"].(float64) != 25 {
		t.Errorf("breakdown[0]: got %v, want Permitted/25", first)
	}
}

func TestNearHandler_EmptyResultIsNonNullArray(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: nil, count: 0}

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
	breakdown, ok := got["statusBreakdown"].([]any)
	if !ok {
		t.Fatalf("statusBreakdown must be a non-null array even when empty, got %T", got["statusBreakdown"])
	}
	if len(breakdown) != 0 {
		t.Errorf("statusBreakdown length: got %d, want 0", len(breakdown))
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

func TestNearHandler_CountErrorReturns500(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), countErr: context.DeadlineExceeded}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("count error: got %d, want 500", rec.Code)
	}
}

func TestNearHandler_DefaultOrderRoutesToRecentNearby(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 5}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	// Absent order param -> recency-ordered RecentNearby (the unchanged default).
	if !store.called || store.nearestCalled {
		t.Errorf("default order must route to RecentNearby, not NearestNearby (called=%v nearest=%v)", store.called, store.nearestCalled)
	}
}

func TestNearHandler_OrderRecencyRoutesToRecentNearby(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 5}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1&order=recency")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	// Explicit order=recency must behave exactly like the default.
	if !store.called || store.nearestCalled {
		t.Errorf("order=recency must route to RecentNearby, not NearestNearby (called=%v nearest=%v)", store.called, store.nearestCalled)
	}
}

func TestNearHandler_OrderDistanceRoutesToNearestNearby(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 5}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5155&lng=-0.0931&radius=4000&order=distance")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	// order=distance must route to the distance-ordered NearestNearby read.
	if !store.nearestCalled {
		t.Fatalf("order=distance must route to NearestNearby")
	}
	// Same scoping/clamping/cap and geo args reach the distance-ordered read.
	if store.lastAuthorityCode != "471" || store.lastCap != 200 {
		t.Errorf("nearest read call: authorityCode=%q cap=%d, want \"471\" 200", store.lastAuthorityCode, store.lastCap)
	}
	if store.lastLat != 51.5155 || store.lastLng != -0.0931 || store.lastRadius != 4000 {
		t.Errorf("nearest geo args: lat=%v lng=%v radius=%v, want 51.5155 -0.0931 4000", store.lastLat, store.lastLng, store.lastRadius)
	}
	// The exact count is still taken over the same scoped, clamped geo window.
	if !store.countCalled || store.lastCountAuthCode != "471" {
		t.Errorf("count call: called=%v auth=%q, want true \"471\"", store.countCalled, store.lastCountAuthCode)
	}
}

func TestNearHandler_UnknownOrderReturns400(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 5}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1&order=banana")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown order: got %d, want 400", rec.Code)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit on an unknown order value")
	}
}

func TestNearHandler_OrderDistanceStillRejectsOutOfRangeCoord(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 5}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=91&lng=-0.1&order=distance")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range lat with order=distance: got %d, want 400", rec.Code)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit on an out-of-range coord, even with order=distance")
	}
}

func TestNearHandler_OrderDistanceStillRejectsNonPositiveRadius(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2), count: 5}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1&radius=0&order=distance")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-positive radius with order=distance: got %d, want 400", rec.Code)
	}
	if store.called || store.countCalled {
		t.Errorf("store must not be hit on a non-positive radius, even with order=distance")
	}
}
