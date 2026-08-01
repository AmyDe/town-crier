//go:build integration

package watchzones

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// Deterministic fixture geometry, mirroring the Phase 0 spatial proof: a fixed
// London centre with zone centres offset due north by a known number of metres.
const (
	wzCentreLon    = -0.1278
	wzCentreLat    = 51.5074
	wzMetresPerLat = 111_320.0
)

func wzLatNorth(metres float64) float64 { return wzCentreLat + metres/wzMetresPerLat }

// uuidN builds a deterministic, ordered UUID so ORDER BY id assertions are stable.
func uuidN(n int) string {
	return fmt.Sprintf("%08d-0000-0000-0000-000000000000", n)
}

// newZonePGStore returns a Postgres-backed watch-zone store over a truncated
// database. Integration tests are NOT parallel: they share the docker-compose DB.
func newZonePGStore(t *testing.T) *PostgresStore {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	return NewPostgresStore(pool)
}

// pgZone constructs a validated watch zone. authorityID must be positive so
// NewWatchZone (and therefore FindZonesContaining hydration) accepts it.
func pgZone(t *testing.T, id, userID, name string, lat, lon, radius float64, authorityID int) WatchZone {
	t.Helper()
	z, err := NewWatchZone(id, userID, name, lat, lon, radius, authorityID,
		time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC), true, false)
	if err != nil {
		t.Fatalf("NewWatchZone(%s): %v", name, err)
	}
	return z
}

func assertZoneEqual(t *testing.T, got, want WatchZone) {
	t.Helper()
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, want.CreatedAt)
	}
	g, w := got, want
	g.CreatedAt, w.CreatedAt = time.Time{}, time.Time{}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("watch zone mismatch:\n got = %+v\nwant = %+v", g, w)
	}
}

func zoneNames(zones []WatchZone) []string {
	names := make([]string, len(zones))
	for i, z := range zones {
		names[i] = z.Name
	}
	return names
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// assertZoneEqualExceptBoundary is assertZoneEqual minus the Boundary field:
// Latitude/Longitude/RadiusMetres round-trip through the geography point
// column bit-for-bit (as assertZoneEqual already proves for circle zones),
// but Boundary round-trips through GeoJSON text (ST_AsGeoJSON's default
// 9-decimal-degree precision), so it needs the tolerance-based
// assertBoundaryNear instead of reflect.DeepEqual.
func assertZoneEqualExceptBoundary(t *testing.T, got, want WatchZone) {
	t.Helper()
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, want.CreatedAt)
	}
	g, w := got, want
	g.CreatedAt, w.CreatedAt = time.Time{}, time.Time{}
	g.Boundary, w.Boundary = nil, nil
	if !reflect.DeepEqual(g, w) {
		t.Errorf("watch zone mismatch (excluding boundary):\n got = %+v\nwant = %+v", g, w)
	}
}

// assertBoundaryNear compares two (already-closed) boundary rings
// vertex-by-vertex within tolMetres, absorbing the precision a GeoJSON text
// round trip through ST_AsGeoJSON's default 9-decimal-degree truncation loses
// relative to the full-precision float64 the domain computed.
func assertBoundaryNear(t *testing.T, got, want Boundary, tolMetres float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("boundary vertex count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		d := haversineMetres(got[i].Latitude, got[i].Longitude, want[i].Latitude, want[i].Longitude)
		if d > tolMetres {
			t.Errorf("vertex %d: got %+v, want %+v (%.6fm apart, tol %.6fm)", i, got[i], want[i], d, tolMetres)
		}
	}
}

// TestWatchZonePostgresStore_SaveGetRoundTrip persists a fully-specified zone and
// reads it back unchanged via both Get and GetByUserID.
func TestWatchZonePostgresStore_SaveGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	want := pgZone(t, uuidN(1), "user-1", "Home", 51.5, -0.12, 500, 33)
	want.PushEnabled = true
	want.EmailInstantEnabled = true

	if err := store.Save(ctx, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(ctx, "user-1", uuidN(1))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertZoneEqual(t, got, want)

	list, err := store.GetByUserID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("GetByUserID len = %d, want 1", len(list))
	}
	assertZoneEqual(t, list[0], want)
}

// TestWatchZonePostgresStore_Get_Miss returns the ErrNotFound sentinel.
func TestWatchZonePostgresStore_Get_Miss(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	_, err := store.Get(ctx, "user-1", uuidN(404))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get miss: got %v, want ErrNotFound", err)
	}
}

// TestWatchZonePostgresStore_Save_UpsertOnID updates the existing row when the
// same id is saved again, rather than inserting a second.
func TestWatchZonePostgresStore_Save_UpsertOnID(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	first := pgZone(t, uuidN(1), "user-1", "Old name", 51.5, -0.12, 500, 10)
	if err := store.Save(ctx, first); err != nil {
		t.Fatalf("Save first: %v", err)
	}
	second := pgZone(t, uuidN(1), "user-1", "New name", 52.0, -1.0, 750, 20)
	if err := store.Save(ctx, second); err != nil {
		t.Fatalf("Save second: %v", err)
	}

	got, err := store.Get(ctx, "user-1", uuidN(1))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertZoneEqual(t, got, second)

	list, err := store.GetByUserID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 zone after upsert, got %d", len(list))
	}
}

// TestWatchZonePostgresStore_GetByUserID_OrderedAndScoped lists a user's zones in
// id order and excludes other users' zones.
func TestWatchZonePostgresStore_GetByUserID_OrderedAndScoped(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	for _, z := range []WatchZone{
		pgZone(t, uuidN(3), "user-1", "third", 51.5, -0.12, 500, 10),
		pgZone(t, uuidN(1), "user-1", "first", 51.5, -0.12, 500, 10),
		pgZone(t, uuidN(2), "user-1", "second", 51.5, -0.12, 500, 10),
		pgZone(t, uuidN(9), "user-2", "other", 51.5, -0.12, 500, 10),
	} {
		if err := store.Save(ctx, z); err != nil {
			t.Fatalf("Save %s: %v", z.Name, err)
		}
	}

	list, err := store.GetByUserID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	assertStrings(t, zoneNames(list), []string{"first", "second", "third"})
}

// TestWatchZonePostgresStore_Delete removes a zone; a miss returns ErrNotFound.
func TestWatchZonePostgresStore_Delete(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	z := pgZone(t, uuidN(1), "user-1", "Home", 51.5, -0.12, 500, 10)
	if err := store.Save(ctx, z); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(ctx, "user-1", uuidN(1)); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, "user-1", uuidN(1)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after delete: got %v, want ErrNotFound", err)
	}
	if err := store.Delete(ctx, "user-1", uuidN(1)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete miss: got %v, want ErrNotFound", err)
	}
}

// TestWatchZonePostgresStore_DeleteAllByUserID clears one user's zones and leaves
// other users untouched.
func TestWatchZonePostgresStore_DeleteAllByUserID(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	for _, z := range []WatchZone{
		pgZone(t, uuidN(1), "user-1", "a", 51.5, -0.12, 500, 10),
		pgZone(t, uuidN(2), "user-1", "b", 51.5, -0.12, 500, 10),
		pgZone(t, uuidN(3), "user-2", "keep", 51.5, -0.12, 500, 10),
	} {
		if err := store.Save(ctx, z); err != nil {
			t.Fatalf("Save %s: %v", z.Name, err)
		}
	}

	if err := store.DeleteAllByUserID(ctx, "user-1"); err != nil {
		t.Fatalf("DeleteAllByUserID: %v", err)
	}
	left, err := store.GetByUserID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetByUserID user-1: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("expected 0 zones for user-1, got %d", len(left))
	}
	other, err := store.GetByUserID(ctx, "user-2")
	if err != nil {
		t.Fatalf("GetByUserID user-2: %v", err)
	}
	if len(other) != 1 {
		t.Errorf("expected user-2's zone untouched, got %d", len(other))
	}
}

// TestWatchZonePostgresStore_DistinctAuthorityIDs returns the deduplicated set of
// authority ids across every user's zones.
func TestWatchZonePostgresStore_DistinctAuthorityIDs(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	for _, z := range []WatchZone{
		pgZone(t, uuidN(1), "user-1", "a", 51.5, -0.12, 500, 10),
		pgZone(t, uuidN(2), "user-1", "b", 51.5, -0.12, 500, 10), // dup authority 10
		pgZone(t, uuidN(3), "user-2", "c", 51.5, -0.12, 500, 20),
		pgZone(t, uuidN(4), "user-3", "d", 51.5, -0.12, 500, 99),
	} {
		if err := store.Save(ctx, z); err != nil {
			t.Fatalf("Save %s: %v", z.Name, err)
		}
	}

	ids, err := store.DistinctAuthorityIDs(ctx)
	if err != nil {
		t.Fatalf("DistinctAuthorityIDs: %v", err)
	}
	if !reflect.DeepEqual(ids, []int{10, 20, 99}) {
		t.Fatalf("DistinctAuthorityIDs = %v, want [10 20 99]", ids)
	}
}

// TestWatchZonePostgresStore_FindZonesContaining proves the notify hot path:
// a zone whose circle contains the point matches; a same-centre narrower zone
// does not (radius-driven, not centre proximity); and matching crosses users and
// authorities (one GiST index, authority-agnostic).
func TestWatchZonePostgresStore_FindZonesContaining(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	// Two zones centred 2 km north of the query point: the wide one's 3 km radius
	// reaches back to the point, the narrow one's 1 km does not.
	wide := pgZone(t, uuidN(1), "user-1", "zone-wide", wzLatNorth(2000), wzCentreLon, 3000, 10)
	narrow := pgZone(t, uuidN(2), "user-1", "zone-narrow", wzLatNorth(2000), wzCentreLon, 1000, 10)
	// A different user, different authority, centred on the point itself.
	other := pgZone(t, uuidN(3), "user-2", "zone-other", wzCentreLat, wzCentreLon, 500, 99)
	for _, z := range []WatchZone{wide, narrow, other} {
		if err := store.Save(ctx, z); err != nil {
			t.Fatalf("Save %s: %v", z.Name, err)
		}
	}

	matched, err := store.FindZonesContaining(ctx, wzCentreLat, wzCentreLon)
	if err != nil {
		t.Fatalf("FindZonesContaining: %v", err)
	}
	assertStrings(t, zoneNames(matched), []string{"zone-wide", "zone-other"})

	// The hydrated zones are valid (positive authority id) and carry their owner.
	for _, z := range matched {
		if z.AuthorityID <= 0 || z.UserID == "" {
			t.Errorf("hydrated zone invalid: %+v", z)
		}
	}
}

// TestWatchZonePostgresStore_Boundary_SaveGetRoundTrip persists a custom-shape
// zone and reads it back via both Get and GetByUserID, proving the boundary
// column round-trips through ST_GeomFromGeoJSON on write and ST_AsGeoJSON on
// read: the ring's vertices, and the derived centroid/enclosing-radius circle
// fields WithBoundary computed before Save, all come back unchanged.
func TestWatchZonePostgresStore_Boundary_SaveGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	base := pgZone(t, uuidN(1), "user-1", "Custom shape", wzCentreLat, wzCentreLon, 1, 10)
	want, err := base.WithBoundary(londonSquare(t))
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}
	if err := store.Save(ctx, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(ctx, "user-1", uuidN(1))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.IsCustomShape() {
		t.Fatal("Get: expected a custom-shape zone")
	}
	assertZoneEqualExceptBoundary(t, got, want)
	assertBoundaryNear(t, got.Boundary, want.Boundary, 0.01)

	list, err := store.GetByUserID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(list) != 1 || !list[0].IsCustomShape() {
		t.Fatalf("GetByUserID: want exactly 1 custom-shape zone, got %+v", list)
	}
	assertZoneEqualExceptBoundary(t, list[0], want)
	assertBoundaryNear(t, list[0].Boundary, want.Boundary, 0.01)
}

// TestWatchZonePostgresStore_Save_BoundaryUpsertTransitions proves Save (the
// store's single upsert path, used for both create and update/patch) persists
// a boundary change on an existing row in both directions: a circle gaining a
// custom shape, and a custom shape reverting to a circle. The revert case is
// the one a naive "only write boundary when non-nil" implementation would get
// wrong -- ON CONFLICT must overwrite the column with SQL NULL, not leave the
// previous polygon stale.
func TestWatchZonePostgresStore_Save_BoundaryUpsertTransitions(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	circle := pgZone(t, uuidN(1), "user-1", "zone", wzCentreLat, wzCentreLon, 2000, 10)
	if err := store.Save(ctx, circle); err != nil {
		t.Fatalf("Save circle: %v", err)
	}
	got, err := store.Get(ctx, "user-1", uuidN(1))
	if err != nil {
		t.Fatalf("Get after circle save: %v", err)
	}
	if got.IsCustomShape() {
		t.Fatal("freshly-saved circle zone must not report IsCustomShape")
	}

	polygon, err := circle.WithBoundary(londonSquare(t))
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}
	if err := store.Save(ctx, polygon); err != nil {
		t.Fatalf("Save polygon (update): %v", err)
	}
	got, err = store.Get(ctx, "user-1", uuidN(1))
	if err != nil {
		t.Fatalf("Get after polygon save: %v", err)
	}
	if !got.IsCustomShape() {
		t.Fatal("zone updated with a boundary must report IsCustomShape")
	}
	assertBoundaryNear(t, got.Boundary, polygon.Boundary, 0.01)

	// Revert to a circle: same row (same id), Boundary cleared.
	reverted := got
	reverted.Boundary = nil
	if err := store.Save(ctx, reverted); err != nil {
		t.Fatalf("Save reverted circle: %v", err)
	}
	got, err = store.Get(ctx, "user-1", uuidN(1))
	if err != nil {
		t.Fatalf("Get after revert: %v", err)
	}
	if got.IsCustomShape() {
		t.Fatal("reverted zone must not report IsCustomShape; boundary column should be NULL")
	}
	if len(got.Boundary) != 0 {
		t.Errorf("reverted zone Boundary = %+v, want empty", got.Boundary)
	}
}

// TestWatchZonePostgresStore_FindZonesContaining_Boundary_PointInsidePolygonMatches
// proves the ST_Covers branch: a point inside a custom-shape zone's polygon
// matches.
func TestWatchZonePostgresStore_FindZonesContaining_Boundary_PointInsidePolygonMatches(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	base := pgZone(t, uuidN(1), "user-1", "polygon", wzCentreLat, wzCentreLon, 1, 10)
	polygon, err := base.WithBoundary(londonSquare(t))
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}
	if err := store.Save(ctx, polygon); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// londonSquare's half-side is ~1000m, so 500m north of centre sits well
	// inside it.
	matched, err := store.FindZonesContaining(ctx, wzLatNorth(500), wzCentreLon)
	if err != nil {
		t.Fatalf("FindZonesContaining: %v", err)
	}
	assertStrings(t, zoneNames(matched), []string{"polygon"})
}

// TestWatchZonePostgresStore_FindZonesContaining_Boundary_PointInEnclosingCircleButOutsidePolygon_NoMatch
// proves ST_Covers rejects a point that a circle-shaped zone with the same
// derived (centroid, enclosing radius) would have matched: the case
// ST_DWithin cannot distinguish, and the whole reason FindZonesContaining
// branches on boundary rather than always matching by circle.
func TestWatchZonePostgresStore_FindZonesContaining_Boundary_PointInEnclosingCircleButOutsidePolygon_NoMatch(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	base := pgZone(t, uuidN(1), "user-1", "polygon", wzCentreLat, wzCentreLon, 1, 10)
	polygon, err := base.WithBoundary(londonSquare(t))
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}
	const testPointMetresNorth = 1200
	if polygon.RadiusMetres <= testPointMetresNorth {
		t.Fatalf("fixture assumption broken: enclosing radius %.2fm must exceed %vm",
			polygon.RadiusMetres, testPointMetresNorth)
	}
	if err := store.Save(ctx, polygon); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 1200m north of centre is inside londonSquare's ~1412.7m enclosing circle
	// (so it WOULD match a circle-shaped zone with the same derived centre and
	// radius) but outside the square itself, whose north edge sits at the
	// square's half-side, ~1000m.
	matched, err := store.FindZonesContaining(ctx, wzLatNorth(testPointMetresNorth), wzCentreLon)
	if err != nil {
		t.Fatalf("FindZonesContaining: %v", err)
	}
	if len(matched) != 0 {
		t.Fatalf("matched = %v, want none (point outside the polygon)", zoneNames(matched))
	}
}

// TestWatchZonePostgresStore_FindZonesContaining_Boundary_AndCircle_BothMatchInOneCall
// proves a single FindZonesContaining call serves both zone shapes together:
// the ST_DWithin and ST_Covers branches of the WHERE clause both contribute
// rows to the same result set.
func TestWatchZonePostgresStore_FindZonesContaining_Boundary_AndCircle_BothMatchInOneCall(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	circle := pgZone(t, uuidN(1), "user-1", "circle", wzCentreLat, wzCentreLon, 2000, 10)
	polygonBase := pgZone(t, uuidN(2), "user-2", "polygon", wzCentreLat, wzCentreLon, 1, 20)
	polygon, err := polygonBase.WithBoundary(londonSquare(t))
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}
	for _, z := range []WatchZone{circle, polygon} {
		if err := store.Save(ctx, z); err != nil {
			t.Fatalf("Save %s: %v", z.Name, err)
		}
	}

	// 500m north of centre matches the polygon (inside londonSquare) and the
	// circle (well within its 2km radius).
	matched, err := store.FindZonesContaining(ctx, wzLatNorth(500), wzCentreLon)
	if err != nil {
		t.Fatalf("FindZonesContaining: %v", err)
	}
	assertStrings(t, zoneNames(matched), []string{"circle", "polygon"})
}

// TestWatchZonePostgresStore_FindZonesContaining_Boundary_ExplainUsesGiSTIndex
// proves the boundary/ST_Covers branch stays index-served rather than
// degrading FindZonesContaining (the notify hot path) to a sequential scan.
// enable_seqscan is disabled for the EXPLAIN so the planner is forced to
// reveal whether an index plan is even possible, rather than preferring a
// sequential scan over the small fixture table (mirrors
// TestPostgresStore_FindClustersInZone_ExplainUsesGiSTIndex in the
// applications package).
func TestWatchZonePostgresStore_FindZonesContaining_Boundary_ExplainUsesGiSTIndex(t *testing.T) {
	ctx := context.Background()
	// Acquire the pool directly (rather than via newZonePGStore) so the same
	// connection can both seed fixtures and run the EXPLAIN below.
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	store := NewPostgresStore(pool)

	circle := pgZone(t, uuidN(1), "user-1", "circle", wzCentreLat, wzCentreLon, 2000, 10)
	polygonBase := pgZone(t, uuidN(2), "user-2", "polygon", wzCentreLat, wzCentreLon, 1, 20)
	polygon, err := polygonBase.WithBoundary(londonSquare(t))
	if err != nil {
		t.Fatalf("WithBoundary: %v", err)
	}
	for _, z := range []WatchZone{circle, polygon} {
		if err := store.Save(ctx, z); err != nil {
			t.Fatalf("Save %s: %v", z.Name, err)
		}
	}

	// SET enable_seqscan = off and the EXPLAIN must run on the SAME connection
	// (the setting is session state), so acquire one pooled connection for both.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SET enable_seqscan = off"); err != nil {
		t.Fatalf("disable seqscan: %v", err)
	}

	rows, err := conn.Query(ctx, "EXPLAIN (ANALYZE, FORMAT TEXT) "+pgFindZonesContainingQuery,
		wzCentreLon, wzLatNorth(500))
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			rows.Close()
			t.Fatalf("scan plan line: %v", err)
		}
		plan.WriteString(line)
		plan.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read plan: %v", err)
	}

	lower := strings.ToLower(plan.String())
	if strings.Contains(lower, "seq scan") {
		t.Errorf("EXPLAIN plan uses a sequential scan with enable_seqscan=off:\n%s", plan.String())
	}
	if !strings.Contains(lower, "watch_zones_boundary_gist") && !strings.Contains(lower, "watch_zones_location_gist") {
		t.Errorf("EXPLAIN plan does not reference either GiST index:\n%s", plan.String())
	}
}
