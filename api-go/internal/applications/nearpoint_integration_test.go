//go:build integration

package applications

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// TestNearPointHandler_Integration_RealPostGIS drives the full HTTP handler —
// param parsing, the store's KNN + ST_DWithin query, and JSON encoding —
// against real PostGIS (ADR 0032). Fakes cannot honestly model true KNN
// distance ordering or ST_DWithin's radius filter, so this pins the whole
// pipeline the way the anonymous iOS browse map actually exercises it:
// nearest-first ordering, the radius filter excluding a far application, and
// keyset pagination across pages with no overlap or gap.
func TestNearPointHandler_Integration_RealPostGIS(t *testing.T) {
	// Not run with t.Parallel(): newAppPGStore's pgtest.New holds a
	// process-wide advisory lock for the test's duration (see pgtest.New).
	store := newAppPGStore(t)
	ctx := context.Background()

	for _, a := range []PlanningApplication{
		at(pgApp("APP-100", 100), 100),
		at(pgApp("APP-500", 100), 500),
		at(pgApp("APP-FAR", 100), 50000), // outside every radius used below
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	mux := http.NewServeMux()
	NearPointRoutes(mux, store, testResolver(), slog.New(slog.DiscardHandler))

	// limit=2: page 1 comes back FULL (2 rows), which always mints a
	// continuation cursor regardless of whether more rows truly exist
	// (FindNearbyPage's documented contract — see collectPage, zonepage.go);
	// page 2 then comes back short of the limit (1 row), which is what
	// actually reports exhaustion with an empty cursor.
	firstURL := "/v1/applications/near-point?lat=" + strconv.FormatFloat(pgCentreLat, 'f', -1, 64) +
		"&lng=" + strconv.FormatFloat(pgCentreLon, 'f', -1, 64) + "&radius=6000&limit=2"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, firstURL, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("page 1 status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var page1 []NearbyResult
	if err := json.Unmarshal(rec.Body.Bytes(), &page1); err != nil {
		t.Fatalf("page 1 body not a bare JSON array: %v; body = %s", err, rec.Body.String())
	}
	if len(page1) != 2 || page1[0].Name != "APP-100" || page1[1].Name != "APP-500" {
		t.Fatalf("page 1 = %+v, want [APP-100, APP-500] nearest-first", page1)
	}
	nextCursor := rec.Header().Get("X-Next-Cursor")
	if nextCursor == "" {
		t.Fatal("expected X-Next-Cursor after a full page")
	}

	secondURL := firstURL + "&cursor=" + nextCursor
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, httptest.NewRequestWithContext(ctx, http.MethodGet, secondURL, nil))

	if rec2.Code != http.StatusOK {
		t.Fatalf("page 2 status = %d, want 200; body = %s", rec2.Code, rec2.Body.String())
	}
	var page2 []NearbyResult
	if err := json.Unmarshal(rec2.Body.Bytes(), &page2); err != nil {
		t.Fatalf("page 2 body not a bare JSON array: %v; body = %s", err, rec2.Body.String())
	}
	// APP-FAR (50km) is excluded by the 6km radius, so page 2 is empty.
	if len(page2) != 0 {
		t.Fatalf("page 2 = %+v, want empty (APP-FAR excluded by radius)", page2)
	}
	if got := rec2.Header().Get("X-Next-Cursor"); got != "" {
		t.Fatalf("expected empty X-Next-Cursor at exhaustion, got %q", got)
	}
}
