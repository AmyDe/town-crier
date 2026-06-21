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
// store boundary), honours cap the way Cosmos TOP @cap does, and serves a
// whole-in-radius status breakdown independently of the bounded read, so a test
// can prove the rendered Total is the sum of those buckets — not the bounded read
// length nor the rendered slice.
type fakeNearStore struct {
	apps         []PlanningApplication
	err          error
	breakdown    []StateCount
	breakdownErr error

	called            bool
	lastAuthorityCode string
	lastLat           float64
	lastLng           float64
	lastRadius        float64
	lastCap           int

	// nearestCalled records whether the distance-ordered read path was taken
	// (order=distance) instead of the recency-ordered RecentNearby default.
	nearestCalled bool

	breakdownCalled       bool
	lastBreakdownAuthCode string
	lastBreakdownLat      float64
	lastBreakdownLng      float64
	lastBreakdownRadius   float64
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

func (f *fakeNearStore) BreakdownNearby(_ context.Context, authorityCode string, lat, lng, radiusMetres float64) ([]StateCount, error) {
	f.breakdownCalled = true
	f.lastBreakdownAuthCode = authorityCode
	f.lastBreakdownLat = lat
	f.lastBreakdownLng = lng
	f.lastBreakdownRadius = radiusMetres
	if f.breakdownErr != nil {
		return nil, f.breakdownErr
	}
	return f.breakdown, nil
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
	// The whole-in-radius breakdown sums to 1234, deliberately distinct from
	// len(apps) (2), to prove total is the sum of the breakdown buckets, not the
	// bounded read length.
	store := &fakeNearStore{
		apps: recentApps(t, 2),
		breakdown: breakdownOf(
			StateCount{AppState: strPtr("Permitted"), Count: 1000},
			StateCount{AppState: strPtr("Rejected"), Count: 234},
		),
	}

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
	// The whole-in-radius breakdown is computed over the same scoped, clamped geo window.
	if !store.breakdownCalled || store.lastBreakdownAuthCode != "471" ||
		store.lastBreakdownLat != 51.5155 || store.lastBreakdownLng != -0.0931 || store.lastBreakdownRadius != 4000 {
		t.Errorf("breakdown call: called=%v auth=%q lat=%v lng=%v radius=%v, want true \"471\" 51.5155 -0.0931 4000",
			store.breakdownCalled, store.lastBreakdownAuthCode, store.lastBreakdownLat, store.lastBreakdownLng, store.lastBreakdownRadius)
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
		t.Errorf("total: got %v, want 1234 (sum of the breakdown buckets)", got["total"])
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
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("401 must be bodyless (backfilled downstream), got %s", rec.Body)
	}
	if store.called || store.breakdownCalled {
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
	if store.called || store.breakdownCalled {
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
	if store.called || store.breakdownCalled {
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
	if store.called || store.breakdownCalled {
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
	if store.called || store.breakdownCalled {
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
	if store.called || store.breakdownCalled {
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
	if store.called || store.breakdownCalled {
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
	if store.called || store.breakdownCalled {
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
	if store.called || store.breakdownCalled {
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
	if store.called || store.breakdownCalled {
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
	if store.called || store.breakdownCalled {
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
	// The clamped 10km radius is what reaches both the read and the breakdown, and
	// what the response echoes.
	if store.lastRadius != 10000 || store.lastBreakdownRadius != 10000 {
		t.Errorf("store radius: read=%v breakdown=%v, want 10000 (clamped to 10km)", store.lastRadius, store.lastBreakdownRadius)
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
	if store.lastRadius != 5000 || store.lastBreakdownRadius != 5000 {
		t.Errorf("store radius: read=%v breakdown=%v, want 5000 (default when unset)", store.lastRadius, store.lastBreakdownRadius)
	}
	got := recentBody(t, rec)
	if got["radius"].(float64) != 5000 {
		t.Errorf("echoed radius: got %v, want 5000", got["radius"])
	}
}

func TestNearHandler_DefaultLimitIs30(t *testing.T) {
	t.Parallel()
	// Breakdown sums to 73, distinct from len(apps) (50) and the limit (30).
	store := &fakeNearStore{
		apps:      recentApps(t, 50),
		breakdown: breakdownOf(StateCount{AppState: strPtr("Permitted"), Count: 73}),
	}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	got := recentBody(t, rec)
	apps := got["applications"].([]any)
	if len(apps) != 30 {
		t.Errorf("applications length: got %d, want 30 (default limit)", len(apps))
	}
	if got["total"].(float64) != 73 {
		t.Errorf("total: got %v, want 73 (sum of breakdown buckets)", got["total"])
	}
}

func TestNearHandler_LimitHardCappedAt100(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{
		apps:      recentApps(t, 150),
		breakdown: breakdownOf(StateCount{AppState: strPtr("Permitted"), Count: 150}),
	}

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
	// The bounded read saturates at cap (200) but the whole-in-radius breakdown
	// sums to far more: total must be that sum, and the render slice clamps to the
	// limit, not the total (the bug the bounded-read clamp guards against).
	store := &fakeNearStore{
		apps: recentApps(t, 200),
		breakdown: breakdownOf(
			StateCount{AppState: strPtr("Permitted"), Count: 4500},
			StateCount{AppState: strPtr("Rejected"), Count: 2002},
		),
	}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	got := recentBody(t, rec)
	if got["total"].(float64) != 6502 {
		t.Errorf("total: got %v, want 6502 (sum of breakdown buckets)", got["total"])
	}
	if apps := got["applications"].([]any); len(apps) != 30 {
		t.Errorf("applications length: got %d, want 30 (default limit, NOT the total)", len(apps))
	}
}

func TestNearHandler_StatusBreakdownIsWholeInRadiusEchoedVerbatim(t *testing.T) {
	t.Parallel()
	// The store's whole-in-radius breakdown sums to 3771 — far beyond the bounded
	// read (40 cards) and the rendered slice (30). The handler must echo the
	// store's breakdown verbatim, order preserved, and set total to its sum.
	store := &fakeNearStore{
		apps: appsByState(t, 25, 15),
		breakdown: breakdownOf(
			StateCount{AppState: strPtr("Permitted"), Count: 2100},
			StateCount{AppState: strPtr("Rejected"), Count: 900},
			StateCount{AppState: strPtr("Conditions"), Count: 771},
		),
	}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	got := recentBody(t, rec)
	breakdown, ok := got["statusBreakdown"].([]any)
	if !ok {
		t.Fatalf("statusBreakdown must be an array, got %T", got["statusBreakdown"])
	}
	if len(breakdown) != 3 {
		t.Fatalf("statusBreakdown length: got %d, want 3 (%v)", len(breakdown), breakdown)
	}
	// Echoed verbatim from the store, order preserved.
	wantStates := []string{"Permitted", "Rejected", "Conditions"}
	wantCounts := []float64{2100, 900, 771}
	for i, row := range breakdown {
		m := row.(map[string]any)
		if m["appState"] != wantStates[i] || m["count"].(float64) != wantCounts[i] {
			t.Errorf("breakdown[%d]: got %v, want %s/%v", i, m, wantStates[i], wantCounts[i])
		}
	}
	// total is the sum of the breakdown buckets (2100+900+771).
	if got["total"].(float64) != 3771 {
		t.Errorf("total: got %v, want 3771 (sum of breakdown buckets)", got["total"])
	}
}

func TestNearHandler_EmptyResultIsNonNullArray(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: nil, breakdown: nil}

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
		t.Errorf("total: got %v, want 0 (empty breakdown sums to 0)", got["total"])
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

func TestNearHandler_BreakdownErrorReturns500(t *testing.T) {
	t.Parallel()
	// The bounded read succeeds but the whole-in-radius breakdown fails -> 500 (no
	// partial total).
	store := &fakeNearStore{apps: recentApps(t, 2), breakdownErr: context.DeadlineExceeded}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("breakdown error: got %d, want 500", rec.Code)
	}
}

func TestNearHandler_DefaultOrderRoutesToRecentNearby(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

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
	store := &fakeNearStore{apps: recentApps(t, 2)}

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
	store := &fakeNearStore{apps: recentApps(t, 2)}

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
	// The whole-in-radius breakdown is still computed over the same scoped, clamped geo window.
	if !store.breakdownCalled || store.lastBreakdownAuthCode != "471" {
		t.Errorf("breakdown call: called=%v auth=%q, want true \"471\"", store.breakdownCalled, store.lastBreakdownAuthCode)
	}
}

func TestNearHandler_UnknownOrderReturns400(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1&order=banana")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown order: got %d, want 400", rec.Code)
	}
	if store.called || store.breakdownCalled {
		t.Errorf("store must not be hit on an unknown order value")
	}
}

func TestNearHandler_OrderDistanceStillRejectsOutOfRangeCoord(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=91&lng=-0.1&order=distance")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range lat with order=distance: got %d, want 400", rec.Code)
	}
	if store.called || store.breakdownCalled {
		t.Errorf("store must not be hit on an out-of-range coord, even with order=distance")
	}
}

func TestNearHandler_OrderDistanceStillRejectsNonPositiveRadius(t *testing.T) {
	t.Parallel()
	store := &fakeNearStore{apps: recentApps(t, 2)}

	rec := serveNear(t, store, "buildkey", "buildkey",
		"/v1/applications/near?authorityId=471&lat=51.5&lng=-0.1&radius=0&order=distance")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-positive radius with order=distance: got %d, want 400", rec.Code)
	}
	if store.called || store.breakdownCalled {
		t.Errorf("store must not be hit on a non-positive radius, even with order=distance")
	}
}
