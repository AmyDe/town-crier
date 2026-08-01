//go:build integration

package applications

import (
	"context"
	"testing"
)

// squareBoundary is a small closed square ring around the shared London centre
// fixture (pgCentreLon/pgCentreLat), [longitude, latitude]-ordered like
// watchzones.Boundary. halfSide is the ring's half-width in degrees, chosen per
// test so a fixture point can be placed inside the square, inside its
// enclosing circle but outside the square, or outside both.
func squareBoundary(halfSide float64) []Coordinate {
	return []Coordinate{
		{Longitude: pgCentreLon + halfSide, Latitude: pgCentreLat + halfSide},
		{Longitude: pgCentreLon - halfSide, Latitude: pgCentreLat + halfSide},
		{Longitude: pgCentreLon - halfSide, Latitude: pgCentreLat - halfSide},
		{Longitude: pgCentreLon + halfSide, Latitude: pgCentreLat - halfSide},
		{Longitude: pgCentreLon + halfSide, Latitude: pgCentreLat + halfSide},
	}
}

// TestPostgresStore_FindInBoundaryPage_CoversPolygonNotEnclosingCircle is the
// honest real-PostGIS proof ADR 0032 requires: a point inside the polygon
// matches, and a point inside the polygon's enclosing circle but outside the
// polygon itself does NOT match -- the exact spatial distinction a fake store
// cannot model.
func TestPostgresStore_FindInBoundaryPage_CoversPolygonNotEnclosingCircle(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	// A ~0.005-degree half-side square (roughly 550m at this latitude). A point
	// due north-east at ~0.02 degrees sits well inside the square's enclosing
	// circle but well outside the square itself.
	boundary := squareBoundary(0.005)

	inside := pgApp("INSIDE", 100)
	inside.Longitude = pgPtr(pgCentreLon + 0.001)
	inside.Latitude = pgPtr(pgCentreLat + 0.001)

	outsideSquareInsideCircle := pgApp("OUTSIDE-SQUARE", 100)
	outsideSquareInsideCircle.Longitude = pgPtr(pgCentreLon + 0.02)
	outsideSquareInsideCircle.Latitude = pgPtr(pgCentreLat + 0.02)

	farOutside := pgApp("FAR-OUTSIDE", 100)
	farOutside.Longitude = pgPtr(pgCentreLon + 1.0)
	farOutside.Latitude = pgPtr(pgCentreLat + 1.0)

	for _, a := range []PlanningApplication{inside, outsideSquareInsideCircle, farOutside} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	page, cursor, err := store.FindInBoundaryPage(ctx, pgCentreLat, pgCentreLon, boundary, 10, "")
	if err != nil {
		t.Fatalf("FindInBoundaryPage: %v", err)
	}
	assertNames(t, appNames(page), []string{"INSIDE"})
	if cursor != "" {
		t.Fatalf("expected empty cursor at exhaustion, got %q", cursor)
	}
}

// TestPostgresStore_FindInBoundaryPage_OnEdgeMatches proves ST_Covers, not
// ST_Contains, is in effect: a point exactly on the boundary edge matches.
func TestPostgresStore_FindInBoundaryPage_OnEdgeMatches(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	boundary := squareBoundary(0.005)

	onEdge := pgApp("ON-EDGE", 100)
	onEdge.Longitude = pgPtr(pgCentreLon)
	onEdge.Latitude = pgPtr(pgCentreLat + 0.005) // exactly on the north edge

	if err := store.Upsert(ctx, onEdge); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	page, _, err := store.FindInBoundaryPage(ctx, pgCentreLat, pgCentreLon, boundary, 10, "")
	if err != nil {
		t.Fatalf("FindInBoundaryPage: %v", err)
	}
	assertNames(t, appNames(page), []string{"ON-EDGE"})
}

// TestPostgresStore_FindInBoundaryPage_Paginates proves the boundary page keeps
// FindNearbyPage's nearest-centroid-first ordering and keyset pagination: a
// full page returns a continuation cursor, and paging with it resumes without
// overlap or gap.
func TestPostgresStore_FindInBoundaryPage_Paginates(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	// A generous square comfortably containing every fixture point, so ordering
	// and pagination -- not containment -- is what this test proves.
	boundary := squareBoundary(0.2)

	for _, a := range []PlanningApplication{
		at(pgApp("APP-100", 100), 100),
		at(pgApp("APP-500", 100), 500),
		at(pgApp("APP-5000", 100), 5000),
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	page1, cursor1, err := store.FindInBoundaryPage(ctx, pgCentreLat, pgCentreLon, boundary, 2, "")
	if err != nil {
		t.Fatalf("FindInBoundaryPage page1: %v", err)
	}
	assertNames(t, appNames(page1), []string{"APP-100", "APP-500"})
	if cursor1 == "" {
		t.Fatal("expected a continuation cursor after a full page")
	}

	page2, cursor2, err := store.FindInBoundaryPage(ctx, pgCentreLat, pgCentreLon, boundary, 2, cursor1)
	if err != nil {
		t.Fatalf("FindInBoundaryPage page2: %v", err)
	}
	assertNames(t, appNames(page2), []string{"APP-5000"})
	if cursor2 != "" {
		t.Fatalf("expected empty cursor at exhaustion, got %q", cursor2)
	}
}
