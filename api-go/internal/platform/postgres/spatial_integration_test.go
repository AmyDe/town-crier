//go:build integration

package postgres_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// Deterministic fixture geometry. The centre is a fixed London point; the
// applications and zones are placed due north of it by a known number of metres,
// so PostGIS's spheroidal distances are exact to within a metre or so. 1 degree
// of latitude is ~111,320 m along a meridian.
const (
	centreLon    = -0.1278
	centreLat    = 51.5074
	metresPerLat = 111_320.0
)

// latNorth returns the latitude that sits `metres` due north of the centre.
func latNorth(metres float64) float64 {
	return centreLat + metres/metresPerLat
}

// seedApplications inserts three applications at ~100 m, ~500 m and ~5 km north
// of the centre. Returns nothing; tests assert against known names.
func seedApplications(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	const insert = `
		INSERT INTO applications
			(planit_name, authority_code, uid, area_name, area_id, location, last_different)
		VALUES
			($1, $2, $3, $4, $5, ST_SetSRID(ST_MakePoint($6, $7), 4326)::geography, $8)`
	lastDifferent := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	apps := []struct {
		name   string
		metres float64
	}{
		{"APP-100", 100},
		{"APP-500", 500},
		{"APP-5000", 5000},
	}
	for _, a := range apps {
		if _, err := pool.Exec(ctx, insert,
			a.name, "100", "uid-"+a.name, "Testshire", 100,
			centreLon, latNorth(a.metres), lastDifferent,
		); err != nil {
			t.Fatalf("seed application %s: %v", a.name, err)
		}
	}
}

// seedZones inserts two watch zones whose centres both sit 2 km north of the
// query point but whose radii differ: the wide zone's circle reaches back to the
// centre, the narrow zone's does not. This proves radius_metres — not just
// centre proximity — drives containment.
func seedZones(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	const insert = `
		INSERT INTO watch_zones
			(id, user_id, name, location, radius_metres)
		VALUES
			(gen_random_uuid(), $1, $2, ST_SetSRID(ST_MakePoint($3, $4), 4326)::geography, $5)`
	zones := []struct {
		name   string
		radius float64
	}{
		{"zone-wide", 3000},   // centre 2 km away, radius 3 km -> contains centre
		{"zone-narrow", 1000}, // centre 2 km away, radius 1 km -> excludes centre
	}
	for _, z := range zones {
		if _, err := pool.Exec(ctx, insert,
			"user-1", z.name, centreLon, latNorth(2000), z.radius,
		); err != nil {
			t.Fatalf("seed zone %s: %v", z.name, err)
		}
	}
}

func queryNames(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) []string {
	t.Helper()
	rows, err := pool.Query(ctx, sql, args...)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("collect rows: %v", err)
	}
	return names
}

func assertNames(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
}

// TestSpatial_RadiusFilter proves ST_DWithin returns exactly the applications
// inside a 1 km circle — the 100 m and 500 m apps, never the 5 km one.
func TestSpatial_RadiusFilter(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	seedApplications(t, ctx, pool)

	const q = `
		SELECT planit_name FROM applications
		WHERE ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, 1000)
		ORDER BY planit_name`
	got := queryNames(t, ctx, pool, q, centreLon, centreLat)
	assertNames(t, got, []string{"APP-100", "APP-500"})
}

// TestSpatial_NearestFirst proves the KNN <-> operator orders by true distance:
// the 100 m app comes before the 500 m app. This server-side ordering is exactly
// what the Cosmos Gateway refuses to do across partitions.
func TestSpatial_NearestFirst(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	seedApplications(t, ctx, pool)

	const q = `
		SELECT planit_name FROM applications
		ORDER BY location <-> ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography
		LIMIT 2`
	got := queryNames(t, ctx, pool, q, centreLon, centreLat)
	assertNames(t, got, []string{"APP-100", "APP-500"})
}

// TestSpatial_AccurateCount proves a cross-cutting count(*) over a spatial filter
// returns the exact number (2). This is the aggregate Cosmos rejects with
// BadRequest substatus 1004 across partitions.
func TestSpatial_AccurateCount(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	seedApplications(t, ctx, pool)

	const q = `
		SELECT count(*) FROM applications
		WHERE ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, 1000)`
	var count int
	if err := pool.QueryRow(ctx, q, centreLon, centreLat).Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
}

// TestSpatial_ZonesContainingPoint proves the notify-path query: given an
// application's point, return exactly the watch zones whose circle contains it.
// Both zones are centred 2 km away, so only the wide-radius zone matches.
func TestSpatial_ZonesContainingPoint(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	seedZones(t, ctx, pool)

	const q = `
		SELECT name FROM watch_zones
		WHERE ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, radius_metres)
		ORDER BY name`
	got := queryNames(t, ctx, pool, q, centreLon, centreLat)
	assertNames(t, got, []string{"zone-wide"})
}
