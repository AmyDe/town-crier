//go:build integration

package applications

import (
	"context"
	"testing"
)

// boundaryClusterQuery is a brief BoundaryClusterQuery builder over the wide
// viewport shared with the circle cluster tests (zoneclusters_integration_test.go),
// varying only the boundary, grid size and status.
func boundaryClusterQuery(boundary []Coordinate, gridSize float64, status string) BoundaryClusterQuery {
	return BoundaryClusterQuery{
		Boundary:        boundary,
		West:            bboxWest,
		South:           bboxSouth,
		East:            bboxEast,
		North:           bboxNorth,
		GridSizeDegrees: gridSize,
		Status:          status,
	}
}

// TestPostgresStore_FindClustersInBoundary_CoversPolygonNotEnclosingCircle
// proves the boundary-scoped cluster aggregation is genuinely polygon-shaped,
// not circle-shaped: a point inside the polygon's enclosing circle but outside
// the polygon itself is excluded from every cluster -- the exact spatial
// distinction a fake store cannot honestly model (ADR 0032).
func TestPostgresStore_FindClustersInBoundary_CoversPolygonNotEnclosingCircle(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	boundary := squareBoundary(0.005)

	inside := clusterApp("INSIDE", 100, 0.001, 0.001)
	outsideSquareInsideCircle := clusterApp("OUTSIDE-SQUARE", 100, 0.02, 0.02)

	for _, a := range []PlanningApplication{inside, outsideSquareInsideCircle} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	clusters, err := store.FindClustersInBoundary(ctx, boundaryClusterQuery(boundary, 1e-6, ""))
	if err != nil {
		t.Fatalf("FindClustersInBoundary: %v", err)
	}
	if got := sumCounts(clusters); got != 1 {
		t.Fatalf("summed cluster count: got %d, want 1 (only the in-polygon point)", got)
	}
	for _, c := range clusters {
		if c.Member == nil || c.Member.Name != "INSIDE" {
			t.Errorf("surviving cluster member: got %+v, want INSIDE", c.Member)
		}
	}
}

// TestPostgresStore_FindClustersInBoundary_CountConservation mirrors
// TestPostgresStore_FindClustersInZone_CountConservation: the summed cluster
// Count equals a plain COUNT(*) over the same in-polygon, in-viewport
// predicate, at several grid sizes -- no double counting and no omission.
func TestPostgresStore_FindClustersInBoundary_CountConservation(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)
	seedSpread(t, store)

	// A generous square comfortably containing every seedSpread point.
	boundary := squareBoundary(0.2)

	for _, grid := range []float64{10.0, 0.01, 1e-6} {
		q := boundaryClusterQuery(boundary, grid, "")
		clusters, err := store.FindClustersInBoundary(ctx, q)
		if err != nil {
			t.Fatalf("grid %v: FindClustersInBoundary: %v", grid, err)
		}
		if got, want := sumCounts(clusters), 8; got != want {
			t.Errorf("grid %v: summed Count = %d, want %d", grid, got, want)
		}
		for _, c := range clusters {
			if got := sumStatusCounts(c); got != c.Count {
				t.Errorf("grid %v: cluster statusCounts sum = %d, want Count %d", grid, got, c.Count)
			}
		}
	}
}

// TestPostgresStore_FindClustersInBoundary_StatusFilter mirrors
// TestPostgresStore_FindClustersInZone_StatusFilter: ?status= restricts the
// boundary-scoped aggregation to matching rows.
func TestPostgresStore_FindClustersInBoundary_StatusFilter(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)
	permitted, rejected := pgPtr("Permitted"), pgPtr("Rejected")
	boundary := squareBoundary(0.2)

	for _, a := range []PlanningApplication{
		withState(clusterApp("P1", 100, 0.0000, 0.0000), permitted),
		withState(clusterApp("P2", 100, 0.0001, 0.0000), permitted),
		withState(clusterApp("R1", 100, 0.0003, 0.0000), rejected),
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	permittedOnly, err := store.FindClustersInBoundary(ctx, boundaryClusterQuery(boundary, 10.0, "Permitted"))
	if err != nil {
		t.Fatalf("filtered: %v", err)
	}
	if got := sumCounts(permittedOnly); got != 2 {
		t.Fatalf("filtered count: got %d, want 2 (only Permitted)", got)
	}
}
