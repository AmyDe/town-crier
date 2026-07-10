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

// clustersWideBBoxParam renders the shared wide viewport (zoneclusters_integration_test.go)
// as a ?bbox= query value.
const clustersWideBBoxParam = "-0.2,51.48,-0.05,51.55"

// clustersBaseURL builds the anonymous clusters URL over the fixture centre
// with a generous radius and the shared wide viewport, so membership below is
// governed by the grid/zoom under test, not by the centre/radius/bbox.
func clustersBaseURL() string {
	return "/v1/applications/clusters?lat=" + strconv.FormatFloat(pgCentreLat, 'f', -1, 64) +
		"&lng=" + strconv.FormatFloat(pgCentreLon, 'f', -1, 64) +
		"&radius=10000&bbox=" + clustersWideBBoxParam
}

func doClustersReq(t *testing.T, mux http.Handler, url string) []Cluster {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, url, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var clusters []Cluster
	if err := json.Unmarshal(rec.Body.Bytes(), &clusters); err != nil {
		t.Fatalf("body not a bare JSON array: %v; body = %s", err, rec.Body.String())
	}
	return clusters
}

// TestClustersHandler_Integration_GridAggregation drives the full anonymous
// HTTP handler — param parsing, the ?zoom= -> grid-size translation, the
// store's real PostGIS grid aggregation, and JSON encoding — against real
// PostGIS (ADR 0032), mirroring TestPostgresStore_FindClustersInZone_ZoomCellBehaviour
// but through the anonymous query path (a caller-supplied centre/radius/bbox
// instead of a stored watch zone): the coarsest zoom collapses the eight
// spread points into one cell, a mid zoom spreads them into four (one per
// tight pair), and the finest zoom (20) yields one cell per point.
func TestClustersHandler_Integration_GridAggregation(t *testing.T) {
	// Not run with t.Parallel(): newAppPGStore's pgtest.New holds a
	// process-wide advisory lock for the test's duration (see pgtest.New).
	store := newAppPGStore(t)
	seedSpread(t, store)

	mux := http.NewServeMux()
	ClustersRoutes(mux, store, testResolver(), slog.New(slog.DiscardHandler))

	coarse := doClustersReq(t, mux, clustersBaseURL()+"&zoom=0")
	if len(coarse) != 1 || coarse[0].Count != 8 {
		t.Fatalf("zoom=0 (coarsest): got %d cells (first count %v), want 1 cell of 8", len(coarse), firstCount(coarse))
	}

	medium := doClustersReq(t, mux, clustersBaseURL()+"&zoom=12")
	if len(medium) != 4 {
		t.Fatalf("zoom=12: got %d cells, want 4 (one per tight pair)", len(medium))
	}
	for _, c := range medium {
		if c.Count != 2 {
			t.Errorf("zoom=12: cell count = %d, want 2", c.Count)
		}
	}
	if !(len(coarse) < len(medium)) {
		t.Errorf("a finer zoom must spread into more cells: zoom=0 -> %d, zoom=12 -> %d", len(coarse), len(medium))
	}

	tiny := doClustersReq(t, mux, clustersBaseURL()+"&zoom=20")
	if len(tiny) != 8 {
		t.Fatalf("zoom=20 (finest): got %d cells, want 8 (one per point)", len(tiny))
	}
	for _, c := range tiny {
		if c.Count != 1 {
			t.Errorf("zoom=20: cell count = %d, want 1", c.Count)
		}
		if c.Member == nil {
			t.Errorf("zoom=20: single-member cell must carry a member id, got nil")
		}
	}
	if !(len(medium) < len(tiny)) {
		t.Errorf("the finest zoom must spread furthest: zoom=12 -> %d, zoom=20 -> %d", len(medium), len(tiny))
	}
}

// TestClustersHandler_Integration_StackedCoalescing drives the full anonymous
// HTTP handler against real PostGIS to prove the unsplittable-cell member list
// (applicationIds) survives the anonymous query path end-to-end: three
// applications at an IDENTICAL coordinate always share a cell (their extent is
// always below FinestGridDegrees, regardless of the request's own zoom), so
// the cell carries the full member list, slug-enriched by the anonymous
// handler — mirroring TestFindClustersInZone_CoincidentApplications_ReturnsMemberList's
// first subtest, through the HTTP handler, with the GH#924 slug enrichment
// layered on top.
func TestClustersHandler_Integration_StackedCoalescing(t *testing.T) {
	// Not run with t.Parallel(): newAppPGStore's pgtest.New holds a
	// process-wide advisory lock for the test's duration (see pgtest.New).
	store := newAppPGStore(t)
	ctx := context.Background()
	// City of London (area id 471, the id testResolver() round-trips to
	// "city-of-london") so the anonymous slug enrichment is exercised for
	// real, not just left empty by a resolver miss.
	for _, name := range []string{"24/001", "24/002", "24/003"} {
		if err := store.Upsert(ctx, clusterApp(name, 471, 0.0, 0.0)); err != nil {
			t.Fatalf("Upsert %s: %v", name, err)
		}
	}

	mux := http.NewServeMux()
	ClustersRoutes(mux, store, testResolver(), slog.New(slog.DiscardHandler))

	// A mid zoom: the coalesce threshold the handler applies is always the
	// finest grid (FinestGridDegrees), independent of this request zoom.
	clusters := doClustersReq(t, mux, clustersBaseURL()+"&zoom=10")
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1 (coincident points share a cell)", len(clusters))
	}
	c := clusters[0]
	if c.Count != 3 {
		t.Fatalf("count: got %d, want 3", c.Count)
	}
	if len(c.Members) != 3 {
		t.Fatalf("applicationIds: got %d entries, want 3", len(c.Members))
	}
	for _, m := range c.Members {
		if m.Authority != "471" {
			t.Errorf("member authority: got %q, want %q", m.Authority, "471")
		}
		if m.AuthoritySlug != "city-of-london" {
			t.Errorf("member authoritySlug: got %q, want %q (anonymous slug enrichment)", m.AuthoritySlug, "city-of-london")
		}
	}
}
