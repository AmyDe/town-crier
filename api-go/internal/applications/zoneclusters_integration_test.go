//go:build integration

package applications

import (
	"context"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// Cluster fixtures reuse the shared London centre (pgCentreLon/pgCentreLat) and
// the pgApp/withState/pgPtr helpers from store_postgres_test.go. Points are placed
// at explicit small degree offsets so the grid-snapping (ST_SnapToGrid) is
// deterministic. The zone query always uses the centre + a generous radius so
// membership is governed by the viewport, not the circle, except where a test
// exercises the radius/bbox predicates directly.

// clusterApp builds an in-radius application at the centre plus (dLng, dLat)
// degrees, in the given authority, with the baseline nullable fields.
func clusterApp(name string, areaID int, dLng, dLat float64) PlanningApplication {
	a := pgApp(name, areaID)
	a.Longitude = pgPtr(pgCentreLon + dLng)
	a.Latitude = pgPtr(pgCentreLat + dLat)
	return a
}

// wideBBox is a viewport comfortably containing every spread/coarse fixture point
// (all within base + ~0.031 deg), as west,south,east,north.
const (
	bboxWest  = -0.2
	bboxSouth = 51.48
	bboxEast  = -0.05
	bboxNorth = 51.55
)

// spreadOffsets are eight distinct (dLng, dLat) points spanning ~0.031 deg of
// longitude in four tight pairs (pair members 0.001 deg apart, pairs 0.01 deg
// apart). Snapping behaviour by cell size:
//   - coarse 10 deg     -> all eight in one cell.
//   - medium 0.01 deg   -> four cells (each pair shares a cell), two members each.
//   - tiny 1e-6 deg     -> eight cells, one member each.
var spreadOffsets = [8][2]float64{
	{0.000, 0.000}, {0.001, 0.000},
	{0.010, 0.000}, {0.011, 0.000},
	{0.020, 0.010}, {0.021, 0.010},
	{0.030, 0.020}, {0.031, 0.020},
}

// seedSpread inserts the eight spread points, all in authority 100.
func seedSpread(t *testing.T, store *PostgresStore) {
	t.Helper()
	ctx := context.Background()
	for i, off := range spreadOffsets {
		a := clusterApp("S"+strconv.Itoa(i), 100, off[0], off[1])
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}
}

// clusterQuery is a brief ClusterQuery builder over the wide viewport and a
// generous radius, varying only the grid size and status.
func clusterQuery(gridSize float64, status string) ClusterQuery {
	return ClusterQuery{
		Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 10000,
		West: bboxWest, South: bboxSouth, East: bboxEast, North: bboxNorth,
		GridSizeDegrees: gridSize, Status: status,
	}
}

// directCount returns COUNT(*) over the same in-radius + in-bbox predicates the
// cluster query applies, the independent reference for count conservation.
func directCount(t *testing.T, store *PostgresStore, q ClusterQuery) int {
	t.Helper()
	var n int
	err := store.db.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM applications "+
			"WHERE ST_DWithin(location, ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography, $3) "+
			"AND location::geometry && ST_MakeEnvelope($4, $5, $6, $7, 4326)",
		q.Longitude, q.Latitude, q.RadiusMetres, q.West, q.South, q.East, q.North).Scan(&n)
	if err != nil {
		t.Fatalf("direct count: %v", err)
	}
	return n
}

func sumCounts(clusters []Cluster) int {
	total := 0
	for _, c := range clusters {
		total += c.Count
	}
	return total
}

func sumStatusCounts(c Cluster) int {
	total := 0
	for _, n := range c.StatusCounts {
		total += n
	}
	return total
}

// TestPostgresStore_FindClustersInZone_CountConservation proves that, at several
// grid sizes, the summed cluster Count equals a plain COUNT(*) over the same
// in-radius + in-bbox predicates — no double counting and no omission.
func TestPostgresStore_FindClustersInZone_CountConservation(t *testing.T) {
	store := newAppPGStore(t)
	seedSpread(t, store)
	ctx := context.Background()

	for _, grid := range []float64{10.0, 0.01, 1e-6} {
		q := clusterQuery(grid, "")
		clusters, err := store.FindClustersInZone(ctx, q)
		if err != nil {
			t.Fatalf("grid %v: FindClustersInZone: %v", grid, err)
		}
		want := directCount(t, store, q)
		if got := sumCounts(clusters); got != want {
			t.Errorf("grid %v: summed Count = %d, want %d (plain COUNT(*))", grid, got, want)
		}
		// Every cluster's per-status breakdown must itself sum to its Count.
		for _, c := range clusters {
			if got := sumStatusCounts(c); got != c.Count {
				t.Errorf("grid %v: cluster statusCounts sum = %d, want Count %d", grid, got, c.Count)
			}
		}
	}
}

// TestPostgresStore_FindClustersInZone_ZoomCellBehaviour proves the grid behaviour
// across zoom: a coarse grid collapses all points into one cell; a finer grid
// spreads them into more cells; the finest grid yields one cell per point, each a
// fully-identified single member.
func TestPostgresStore_FindClustersInZone_ZoomCellBehaviour(t *testing.T) {
	store := newAppPGStore(t)
	seedSpread(t, store)
	ctx := context.Background()

	coarse, err := store.FindClustersInZone(ctx, clusterQuery(10.0, ""))
	if err != nil {
		t.Fatalf("coarse: %v", err)
	}
	if len(coarse) != 1 || coarse[0].Count != 8 {
		t.Fatalf("coarse grid: got %d cells (first count %v), want 1 cell of 8", len(coarse), firstCount(coarse))
	}

	medium, err := store.FindClustersInZone(ctx, clusterQuery(0.01, ""))
	if err != nil {
		t.Fatalf("medium: %v", err)
	}
	if len(medium) != 4 {
		t.Fatalf("medium grid: got %d cells, want 4 (one per tight pair)", len(medium))
	}
	for _, c := range medium {
		if c.Count != 2 {
			t.Errorf("medium grid: cell count = %d, want 2", c.Count)
		}
	}
	if !(len(coarse) < len(medium)) {
		t.Errorf("a finer grid must spread into more cells: coarse=%d medium=%d", len(coarse), len(medium))
	}

	tiny, err := store.FindClustersInZone(ctx, clusterQuery(1e-6, ""))
	if err != nil {
		t.Fatalf("tiny: %v", err)
	}
	if len(tiny) != 8 {
		t.Fatalf("tiny grid: got %d cells, want 8 (one per point)", len(tiny))
	}
	for _, c := range tiny {
		if c.Count != 1 {
			t.Errorf("tiny grid: cell count = %d, want 1", c.Count)
		}
		if c.Member == nil {
			t.Errorf("tiny grid: single-member cell must carry a member id, got nil")
		}
	}
	if !(len(medium) < len(tiny)) {
		t.Errorf("the finest grid must spread furthest: medium=%d tiny=%d", len(medium), len(tiny))
	}
}

func firstCount(clusters []Cluster) any {
	if len(clusters) == 0 {
		return "none"
	}
	return clusters[0].Count
}

// TestPostgresStore_FindClustersInZone_SingleMemberIdentity proves a single-member
// cell carries the member's {authority, name} and its app_state in statusCounts,
// including the NULL-app_state -> "Unknown" folding.
func TestPostgresStore_FindClustersInZone_SingleMemberIdentity(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()
	for _, a := range []PlanningApplication{
		withState(clusterApp("PERM", 100, 0.000, 0.000), pgPtr("Permitted")),
		clusterApp("NOSTATE", 100, 0.020, 0.020), // NULL app_state
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	clusters, err := store.FindClustersInZone(ctx, clusterQuery(1e-6, ""))
	if err != nil {
		t.Fatalf("FindClustersInZone: %v", err)
	}
	if len(clusters) != 2 {
		t.Fatalf("got %d clusters, want 2", len(clusters))
	}
	byName := map[string]Cluster{}
	for _, c := range clusters {
		if c.Member == nil {
			t.Fatalf("expected every cell to carry a single member, got nil for %+v", c)
		}
		byName[c.Member.Name] = c
	}

	perm, ok := byName["PERM"]
	if !ok {
		t.Fatal("missing PERM cluster")
	}
	if perm.Member.Authority != "100" {
		t.Errorf("PERM authority: got %q, want %q (area_id as decimal string)", perm.Member.Authority, "100")
	}
	if perm.Count != 1 || perm.StatusCounts["Permitted"] != 1 || len(perm.StatusCounts) != 1 {
		t.Errorf("PERM: got count=%d statusCounts=%v, want count=1 {Permitted:1}", perm.Count, perm.StatusCounts)
	}

	noState, ok := byName["NOSTATE"]
	if !ok {
		t.Fatal("missing NOSTATE cluster")
	}
	if noState.StatusCounts["Unknown"] != 1 || len(noState.StatusCounts) != 1 {
		t.Errorf("NOSTATE: got statusCounts=%v, want {Unknown:1} (NULL app_state folds to Unknown)", noState.StatusCounts)
	}
}

// TestPostgresStore_FindClustersInZone_MultiMemberBreakdown proves a multi-member
// cell carries a nil member and a per-status breakdown that sums to the count.
func TestPostgresStore_FindClustersInZone_MultiMemberBreakdown(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()
	// Three points within a tiny span so any coarse grid keeps them in one cell.
	for _, a := range []PlanningApplication{
		withState(clusterApp("M1", 100, 0.0000, 0.0000), pgPtr("Permitted")),
		withState(clusterApp("M2", 100, 0.0001, 0.0000), pgPtr("Permitted")),
		withState(clusterApp("M3", 100, 0.0002, 0.0001), pgPtr("Rejected")),
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	clusters, err := store.FindClustersInZone(ctx, clusterQuery(10.0, ""))
	if err != nil {
		t.Fatalf("FindClustersInZone: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1 coarse cell", len(clusters))
	}
	c := clusters[0]
	if c.Member != nil {
		t.Errorf("multi-member cell must have a nil member, got %+v", c.Member)
	}
	if c.Count != 3 {
		t.Errorf("count: got %d, want 3", c.Count)
	}
	if c.StatusCounts["Permitted"] != 2 || c.StatusCounts["Rejected"] != 1 {
		t.Errorf("statusCounts: got %v, want {Permitted:2, Rejected:1}", c.StatusCounts)
	}
	if sumStatusCounts(c) != c.Count {
		t.Errorf("statusCounts must sum to count: sum=%d count=%d", sumStatusCounts(c), c.Count)
	}
}

// TestPostgresStore_FindClustersInZone_StatusFilter proves ?status= restricts the
// aggregation to matching rows: the filtered counts reflect only that app_state.
func TestPostgresStore_FindClustersInZone_StatusFilter(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()
	permitted, rejected := pgPtr("Permitted"), pgPtr("Rejected")
	for _, a := range []PlanningApplication{
		withState(clusterApp("P1", 100, 0.0000, 0.0000), permitted),
		withState(clusterApp("P2", 100, 0.0001, 0.0000), permitted),
		withState(clusterApp("P3", 100, 0.0002, 0.0001), permitted),
		withState(clusterApp("R1", 100, 0.0003, 0.0000), rejected),
		withState(clusterApp("R2", 100, 0.0004, 0.0001), rejected),
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	all, err := store.FindClustersInZone(ctx, clusterQuery(10.0, ""))
	if err != nil {
		t.Fatalf("unfiltered: %v", err)
	}
	if got := sumCounts(all); got != 5 {
		t.Fatalf("unfiltered count: got %d, want 5", got)
	}

	permittedOnly, err := store.FindClustersInZone(ctx, clusterQuery(10.0, "Permitted"))
	if err != nil {
		t.Fatalf("filtered: %v", err)
	}
	if len(permittedOnly) != 1 {
		t.Fatalf("filtered: got %d clusters, want 1", len(permittedOnly))
	}
	c := permittedOnly[0]
	if c.Count != 3 {
		t.Errorf("filtered count: got %d, want 3 (only Permitted)", c.Count)
	}
	if len(c.StatusCounts) != 1 || c.StatusCounts["Permitted"] != 3 {
		t.Errorf("filtered statusCounts: got %v, want {Permitted:3}", c.StatusCounts)
	}
}

// TestPostgresStore_FindClustersInZone_Centroid proves the cell centroid is the
// arithmetic mean of its members' points, lies within the grid cell, and lies
// within the convex hull of the members.
func TestPostgresStore_FindClustersInZone_Centroid(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()
	const grid = 0.01
	// Three non-collinear points comfortably inside one 0.01-deg cell.
	offs := [3][2]float64{{0.0000, 0.0000}, {0.0002, 0.0000}, {0.0001, 0.0002}}
	for i, off := range offs {
		a := clusterApp("C"+strconv.Itoa(i), 100, off[0], off[1])
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	clusters, err := store.FindClustersInZone(ctx, clusterQuery(grid, ""))
	if err != nil {
		t.Fatalf("FindClustersInZone: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1", len(clusters))
	}
	c := clusters[0]

	var meanLng, meanLat float64
	for _, off := range offs {
		meanLng += pgCentreLon + off[0]
		meanLat += pgCentreLat + off[1]
	}
	meanLng /= 3
	meanLat /= 3
	if !floatNear(c.Longitude, meanLng, 1e-9) || !floatNear(c.Latitude, meanLat, 1e-9) {
		t.Errorf("centroid: got (%v, %v), want arithmetic mean (%v, %v)", c.Longitude, c.Latitude, meanLng, meanLat)
	}

	// Within the grid cell: the centroid is within +/- grid/2 of the cell node.
	nodeLng := math.Round((pgCentreLon+offs[0][0])/grid) * grid
	nodeLat := math.Round((pgCentreLat+offs[0][1])/grid) * grid
	if math.Abs(c.Longitude-nodeLng) > grid/2+1e-9 || math.Abs(c.Latitude-nodeLat) > grid/2+1e-9 {
		t.Errorf("centroid (%v, %v) outside cell node (%v, %v) +/- %v", c.Longitude, c.Latitude, nodeLng, nodeLat, grid/2)
	}

	// Within the convex hull of the members (PostGIS confirms the spatial fact).
	var inHull bool
	hullSQL := "SELECT ST_Within(" +
		"ST_SetSRID(ST_MakePoint($1, $2), 4326), " +
		"ST_ConvexHull(ST_Collect(location::geometry))) " +
		"FROM applications"
	if err := store.db.QueryRow(ctx, hullSQL, c.Longitude, c.Latitude).Scan(&inHull); err != nil {
		t.Fatalf("convex-hull check: %v", err)
	}
	if !inHull {
		t.Errorf("centroid (%v, %v) is not within the convex hull of its members", c.Longitude, c.Latitude)
	}
}

// TestPostgresStore_FindClustersInZone_RespectsRadiusAndBBox proves both spatial
// predicates bound the aggregation: a point inside the radius but outside the
// viewport, and a point inside the viewport but outside the radius, are both
// excluded; only the point inside both survives. The viewport and circle here
// partially overlap so each predicate is exercised independently.
func TestPostgresStore_FindClustersInZone_RespectsRadiusAndBBox(t *testing.T) {
	store := newAppPGStore(t)
	ctx := context.Background()
	for _, a := range []PlanningApplication{
		clusterApp("INBOTH", 100, 0.0278, 0.0126),   // ~2.4 km, inside the NE viewport
		clusterApp("RADONLY", 100, 0.0000, -0.0274), // ~3.0 km but south of the viewport
		clusterApp("BBOXONLY", 100, 0.0678, 0.0826), // inside the viewport but ~10 km out
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	q := ClusterQuery{
		Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 5000,
		West: pgCentreLon, South: pgCentreLat, East: 0.0, North: 51.60,
		GridSizeDegrees: 1e-6,
	}
	clusters, err := store.FindClustersInZone(ctx, q)
	if err != nil {
		t.Fatalf("FindClustersInZone: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("got %d clusters, want 1 (only the in-radius, in-viewport point)", len(clusters))
	}
	if clusters[0].Member == nil || clusters[0].Member.Name != "INBOTH" {
		t.Errorf("surviving cluster: got %+v, want the INBOTH member", clusters[0].Member)
	}
}

// TestPostgresStore_FindClustersInZone_ExplainUsesGiSTIndex proves the existing
// GiST index (applications_location_gist) serves the ST_DWithin radius predicate
// of the cluster query — so no new migration/index is needed. enable_seqscan is
// disabled for the EXPLAIN so the planner is forced to reveal whether the index is
// usable, rather than preferring a sequential scan over the small fixture table.
func TestPostgresStore_FindClustersInZone_ExplainUsesGiSTIndex(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	store := NewPostgresStore(pool)
	seedSpread(t, store)
	ctx := context.Background()

	q := clusterQuery(0.01, "")
	args := append([]any{}, q.Longitude, q.Latitude, q.RadiusMetres, q.GridSizeDegrees, q.West, q.South, q.East, q.North)

	// SET enable_seqscan = off and the EXPLAIN must run on the SAME connection
	// (the setting is session state), so acquire one pooled connection for both.
	// Disabling seqscan forces the planner to reveal whether the GiST index is
	// usable for the predicate, rather than preferring a sequential scan over the
	// small fixture table.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SET enable_seqscan = off"); err != nil {
		t.Fatalf("disable seqscan: %v", err)
	}

	rows, err := conn.Query(ctx, "EXPLAIN (ANALYZE, FORMAT TEXT) "+clusterQueryAllStatuses, args...)
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

	if !strings.Contains(strings.ToLower(plan.String()), "applications_location_gist") {
		t.Errorf("EXPLAIN plan does not use the GiST index applications_location_gist:\n%s", plan.String())
	}
}

func floatNear(a, b, tol float64) bool { return math.Abs(a-b) <= tol }
