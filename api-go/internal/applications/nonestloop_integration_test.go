//go:build integration

package applications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

const (
	fixtureGrid   = 0.01
	fixtureRadius = 100_000.0
	fixtureWest   = -0.25
	fixtureSouth  = 51.45
	fixtureEast   = 0.12
	fixtureNorth  = 51.8
)

const fixtureInsertPrefix = `INSERT INTO applications
	(planit_name, authority_code, uid, area_name, area_id, app_state, location, last_different) `

// fixtureBlocks seed a 750-cell lattice of 20,000 jittered rows (splittable
// cells) plus the special cells the old-vs-new equality tests must cover.
var fixtureBlocks = []string{
	// lattice: 25 x 30 cells at 0.01 degree spacing, jitter well inside a cell.
	`SELECT 'L' || i, (100 + i % 3)::text, 'uid-L' || i, 'Test', 100 + i % 3,
		CASE i % 5 WHEN 0 THEN 'Undecided' WHEN 1 THEN 'Permitted' WHEN 2 THEN 'Conditions'
			WHEN 3 THEN 'Rejected' ELSE NULL END,
		ST_SetSRID(ST_MakePoint(
			-0.2 + (i % 25) * 0.01 + ((i * 7919) % 1000)::float8 / 1000 * 0.006 - 0.003,
			51.5 + ((i / 25) % 30) * 0.01 + ((i * 104729) % 1000)::float8 / 1000 * 0.006 - 0.003
		), 4326)::geography, now()
	FROM generate_series(0, 19999) i`,
	// A: coincident cell of 3, mixed authorities, one NULL state.
	`SELECT 'A-' || i, CASE WHEN i = 1 THEN '200' ELSE '100' END, 'uid-A' || i, 'Test', 100, (ARRAY['Permitted', 'Rejected', NULL])[i],
		ST_SetSRID(ST_MakePoint(0.0612, 51.60), 4326)::geography, now()
	FROM generate_series(1, 3) i`,
	// B: coincident cell of 60 across two authorities, half NULL state.
	`SELECT 'B-' || lpad(i::text, 3, '0'), CASE WHEN i <= 30 THEN '100' ELSE '200' END, 'uid-B' || i, 'Test', 100,
		CASE WHEN i % 2 = 1 THEN 'Permitted' ELSE NULL END,
		ST_SetSRID(ST_MakePoint(0.0712, 51.60), 4326)::geography, now()
	FROM generate_series(1, 60) i`,
	// C: coincident pair, NULL app_state next to a real one.
	`SELECT 'C-' || i, '100', 'uid-C' || i, 'Test', 100, CASE WHEN i = 1 THEN NULL ELSE 'Permitted' END,
		ST_SetSRID(ST_MakePoint(0.0812, 51.60), 4326)::geography, now()
	FROM generate_series(1, 2) i`,
	// D: same planit_name in two authorities, coincident.
	`SELECT 'DUP/1', a, 'uid-D' || a, 'Test', a::int, 'Undecided',
		ST_SetSRID(ST_MakePoint(0.0912, 51.60), 4326)::geography, now()
	FROM unnest(ARRAY['100', '200']) a`,
	// E: two single-member cells.
	`SELECT 'E-' || i, '100', 'uid-E' || i, 'Test', 100, CASE WHEN i = 1 THEN 'Permitted' ELSE NULL END,
		ST_SetSRID(ST_MakePoint(0.0612 + (i - 1) * 0.01, 51.70), 4326)::geography, now()
	FROM generate_series(1, 2) i`,
	// F: four points 1e-5 apart, below the coalesce threshold but not coincident.
	`SELECT 'F-' || i, '100', 'uid-F' || i, 'Test', 100, 'Rejected',
		ST_SetSRID(ST_MakePoint(0.1012 + i * 0.00001, 51.60), 4326)::geography, now()
	FROM generate_series(1, 4) i`,
}

// seedClusterFixture truncates, seeds the shared cluster fixture in a handful of
// INSERT ... SELECT statements and runs ANALYZE so the planner sees real sizes.
func seedClusterFixture(t *testing.T) (*pgxpool.Pool, *PostgresStore) {
	t.Helper()
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")

	for i, block := range fixtureBlocks {
		if _, err := pool.Exec(ctx, fixtureInsertPrefix+block); err != nil {
			t.Fatalf("seed fixture block %d: %v", i, err)
		}
	}
	if _, err := pool.Exec(ctx, "ANALYZE applications"); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	var n int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM applications").Scan(&n); err != nil {
		t.Fatalf("count fixture: %v", err)
	}
	if n < 20_000 {
		t.Fatalf("fixture has %d rows, want >= 20000", n)
	}
	return pool, NewPostgresStore(pool)
}

func fixtureZoneQuery(status string) ClusterQuery {
	return ClusterQuery{
		Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: fixtureRadius,
		West: fixtureWest, South: fixtureSouth, East: fixtureEast, North: fixtureNorth,
		GridSizeDegrees: fixtureGrid, Status: status, CoalesceThresholdDegrees: finestGridDegrees,
	}
}

func fixtureBoundaryQuery(status string) BoundaryClusterQuery {
	return BoundaryClusterQuery{
		Boundary: squareBoundary(0.5),
		West:     fixtureWest, South: fixtureSouth, East: fixtureEast, North: fixtureNorth,
		GridSizeDegrees: fixtureGrid, Status: status, CoalesceThresholdDegrees: finestGridDegrees,
	}
}

// zoneQueryAndArgs mirrors FindClustersInZone's query selection and parameters.
func zoneQueryAndArgs(q ClusterQuery) (string, []any) {
	query := clusterQueryAllStatuses
	args := []any{q.Longitude, q.Latitude, q.RadiusMetres, q.GridSizeDegrees, q.West, q.South, q.East, q.North, q.CoalesceThresholdDegrees}
	if q.Status != "" {
		query = clusterQueryByStatus
		args = append(args, q.Status)
	}
	return query, args
}

// boundaryQueryAndArgs mirrors FindClustersInBoundary's query selection and parameters.
func boundaryQueryAndArgs(t *testing.T, q BoundaryClusterQuery) (string, []any) {
	t.Helper()
	boundaryJSON, err := encodeBoundaryGeoJSON(q.Boundary)
	if err != nil {
		t.Fatalf("encode boundary: %v", err)
	}
	query := boundaryClusterQueryAllStatuses
	args := []any{boundaryJSON, q.GridSizeDegrees, q.West, q.South, q.East, q.North, q.CoalesceThresholdDegrees}
	if q.Status != "" {
		query = boundaryClusterQueryByStatus
		args = append(args, q.Status)
	}
	return query, args
}

func collectClusters(ctx context.Context, t *testing.T, db querier, query string, args ...any) []Cluster {
	t.Helper()
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		t.Fatalf("query clusters: %v", err)
	}
	clusters, err := pgx.CollectRows(rows, scanClusterRow)
	if err != nil {
		t.Fatalf("collect clusters: %v", err)
	}
	return clusters
}

func sortClusters(clusters []Cluster) {
	key := func(v float64) int64 { return int64(math.Round(v * 1e6)) }
	sort.Slice(clusters, func(i, j int) bool {
		a, b := clusters[i], clusters[j]
		if key(a.Latitude) != key(b.Latitude) {
			return key(a.Latitude) < key(b.Latitude)
		}
		return key(a.Longitude) < key(b.Longitude)
	})
}

// assertClustersEqual compares two result sets cell by cell: exact on counts,
// status counts, member lists and single-member identity; centroids within 1e-9.
func assertClustersEqual(t *testing.T, got, want []Cluster) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("cell count: got %d, want %d", len(got), len(want))
	}
	got = append([]Cluster(nil), got...)
	want = append([]Cluster(nil), want...)
	sortClusters(got)
	sortClusters(want)
	diffs := 0
	for i := range want {
		g, w := got[i], want[i]
		if math.Abs(g.Latitude-w.Latitude) > 1e-9 || math.Abs(g.Longitude-w.Longitude) > 1e-9 {
			t.Errorf("cell %d centroid: got (%v, %v), want (%v, %v)", i, g.Latitude, g.Longitude, w.Latitude, w.Longitude)
			diffs++
		}
		if g.Count != w.Count {
			t.Errorf("cell %d (%v, %v) count: got %d, want %d", i, w.Latitude, w.Longitude, g.Count, w.Count)
			diffs++
		}
		if !reflect.DeepEqual(g.StatusCounts, w.StatusCounts) {
			t.Errorf("cell %d (%v, %v) statusCounts: got %v, want %v", i, w.Latitude, w.Longitude, g.StatusCounts, w.StatusCounts)
			diffs++
		}
		if !reflect.DeepEqual(g.Member, w.Member) {
			t.Errorf("cell %d (%v, %v) member: got %+v, want %+v", i, w.Latitude, w.Longitude, g.Member, w.Member)
			diffs++
		}
		if !reflect.DeepEqual(g.Members, w.Members) {
			t.Errorf("cell %d (%v, %v) members: got %v, want %v", i, w.Latitude, w.Longitude, g.Members, w.Members)
			diffs++
		}
		if diffs > 20 {
			t.Fatalf("too many differences, stopping")
		}
	}
}

// assertFixtureCoverage proves the unfiltered result really contains every case
// the equality test is meant to exercise, so the comparison cannot pass vacuously.
func assertFixtureCoverage(ctx context.Context, t *testing.T, pool *pgxpool.Pool, clusters []Cluster) {
	t.Helper()
	if len(clusters) < 700 || len(clusters) >= 1000 {
		t.Fatalf("cells: got %d, want 700..999", len(clusters))
	}
	var (
		triple, big, withUnknown, dupName, single, splittable, spread bool
		splittableCells                                               int
	)
	for _, c := range clusters {
		switch {
		case c.Count == 3 && len(c.Members) == 3:
			triple = true
		case c.Count == 60 && len(c.Members) == 50:
			big = true
			assertFirstFiftyOfBigCell(ctx, t, pool, c)
		case c.Count == 4 && len(c.Members) == 4:
			spread = true
		case c.Count == 1 && c.Member != nil:
			single = true
		case c.Count > 1 && c.Members == nil:
			splittable = true
			splittableCells++
		}
		if c.StatusCounts["Unknown"] > 0 {
			withUnknown = true
		}
		if len(c.Members) == 2 && c.Members[0].Name == c.Members[1].Name && c.Members[0].Authority != c.Members[1].Authority {
			dupName = true
		}
	}
	for name, ok := range map[string]bool{
		"coincident cell of 3": triple, "coincident cell of 60": big, "NULL app_state cell": withUnknown,
		"same name in two authorities": dupName, "single-member cell": single,
		"splittable multi-member cell": splittable, "near-coincident unsplittable cell": spread,
	} {
		if !ok {
			t.Errorf("fixture coverage: no %s in result", name)
		}
	}
	if splittableCells < 500 {
		t.Errorf("splittable multi-member cells: got %d, want >= 500", splittableCells)
	}
}

func assertFirstFiftyOfBigCell(ctx context.Context, t *testing.T, pool *pgxpool.Pool, c Cluster) {
	t.Helper()
	rows, err := pool.Query(ctx,
		"SELECT authority_code, planit_name FROM applications WHERE planit_name LIKE 'B-%' ORDER BY authority_code, planit_name LIMIT 50")
	if err != nil {
		t.Fatalf("expected first 50: %v", err)
	}
	want, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (PlanningApplicationID, error) {
		var id PlanningApplicationID
		err := r.Scan(&id.Authority, &id.Name)
		return id, err
	})
	if err != nil {
		t.Fatalf("collect expected first 50: %v", err)
	}
	if !reflect.DeepEqual(c.Members, want) {
		t.Errorf("60-member cell members: got %v, want first 50 by (authority, name) %v", c.Members, want)
	}
}

type planNode struct {
	NodeType           string     `json:"Node Type"`
	ParentRelationship string     `json:"Parent Relationship"`
	Plans              []planNode `json:"Plans"`
}

func (n planNode) flatten(out *[]planNode) {
	*out = append(*out, n)
	for _, c := range n.Plans {
		c.flatten(out)
	}
}

func parsePlan(t *testing.T, raw []byte) []planNode {
	t.Helper()
	var doc []struct {
		Plan planNode `json:"Plan"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc) != 1 {
		t.Fatalf("parse EXPLAIN JSON (len %d): %v\n%s", len(doc), err, raw)
	}
	var nodes []planNode
	doc[0].Plan.flatten(&nodes)
	return nodes
}

// planViolations lists nested loops and correlated SubPlans; CTE InitPlans are allowed.
func planViolations(nodes []planNode) []string {
	var out []string
	for _, n := range nodes {
		if n.NodeType == "Nested Loop" {
			out = append(out, "Nested Loop")
		}
		if n.ParentRelationship == "SubPlan" {
			out = append(out, "SubPlan under "+n.NodeType)
		}
	}
	return out
}

func planShape(nodes []planNode) string {
	types := make([]string, len(nodes))
	for i, n := range nodes {
		types[i] = n.NodeType
	}
	return strings.Join(types, ">")
}

func explainJSON(t *testing.T, db interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, query string, args ...any) []byte {
	t.Helper()
	rows, err := db.Query(context.Background(), "EXPLAIN (FORMAT JSON) "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	raw, err := pgx.CollectOneRow(rows, func(r pgx.CollectableRow) ([]byte, error) {
		var b []byte
		err := r.Scan(&b)
		return b, err
	})
	if err != nil {
		t.Fatalf("read EXPLAIN: %v", err)
	}
	return raw
}

func TestPlanViolations_IgnoresCTEInitPlansButFlagsNestedLoopAndSubPlan(t *testing.T) {
	clean := `[{"Plan":{"Node Type":"Hash Join","Plans":[
		{"Node Type":"CTE Scan","Parent Relationship":"InitPlan","Subplan Name":"CTE filtered"},
		{"Node Type":"Seq Scan","Parent Relationship":"Outer"}]}}]`
	if v := planViolations(parsePlan(t, []byte(clean))); len(v) != 0 {
		t.Errorf("clean plan flagged: %v", v)
	}
	dirty := `[{"Plan":{"Node Type":"Nested Loop","Plans":[
		{"Node Type":"Result","Parent Relationship":"SubPlan"}]}}]`
	if v := planViolations(parsePlan(t, []byte(dirty))); len(v) != 2 {
		t.Errorf("dirty plan violations: got %v, want Nested Loop and SubPlan", v)
	}
}

func TestPostgresStore_ClusterQueries_ExplainHasNoNestedLoopOrSubPlan(t *testing.T) {
	_, store := seedClusterFixture(t)
	ctx := context.Background()

	cases := map[string]func(status string) (string, []any){
		"zone": func(status string) (string, []any) { return zoneQueryAndArgs(fixtureZoneQuery(status)) },
		"boundary": func(status string) (string, []any) {
			return boundaryQueryAndArgs(t, fixtureBoundaryQuery(status))
		},
	}
	for name, build := range cases {
		for _, status := range []string{"", "Permitted"} {
			t.Run(fmt.Sprintf("%s/status=%q", name, status), func(t *testing.T) {
				query, args := build(status)
				var raw []byte
				err := store.withNoNestLoop(ctx, func(tx pgx.Tx) error {
					rows, err := tx.Query(ctx, "EXPLAIN (ANALYZE, FORMAT JSON) "+query, append([]any{pgx.QueryExecModeExec}, args...)...)
					if err != nil {
						return err
					}
					raw, err = pgx.CollectOneRow(rows, func(r pgx.CollectableRow) ([]byte, error) {
						var b []byte
						err := r.Scan(&b)
						return b, err
					})
					return err
				})
				if err != nil {
					t.Fatalf("explain through withNoNestLoop: %v", err)
				}
				nodes := parsePlan(t, raw)
				if v := planViolations(nodes); len(v) != 0 {
					t.Errorf("plan has %v:\n%s", v, raw)
				}
			})
		}
	}
}

func TestWithNoNestLoop_SetsOffInsideAndRestoresOnSameConnection(t *testing.T) {
	pool, _ := seedClusterFixture(t)
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()
	store := NewPostgresStore(conn)

	show := func(db querier) string {
		var v string
		if err := db.QueryRow(ctx, "SHOW enable_nestloop").Scan(&v); err != nil {
			t.Fatalf("SHOW enable_nestloop: %v", err)
		}
		return v
	}
	if got := show(conn); got != "on" {
		t.Fatalf("before: enable_nestloop = %q, want on", got)
	}
	var inside string
	if err := store.withNoNestLoop(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SHOW enable_nestloop").Scan(&inside)
	}); err != nil {
		t.Fatalf("withNoNestLoop: %v", err)
	}
	if inside != "off" {
		t.Errorf("inside: enable_nestloop = %q, want off", inside)
	}
	if got := show(conn); got != "on" {
		t.Errorf("after: enable_nestloop = %q, want on on the same connection", got)
	}
}

func TestWithNoNestLoop_RejectsWrites(t *testing.T) {
	_, store := seedClusterFixture(t)
	ctx := context.Background()
	err := store.withNoNestLoop(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "DELETE FROM applications")
		return err
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "25006" {
		t.Fatalf("write in read-only tx: got %v, want SQLSTATE 25006", err)
	}
	var n int
	if err := store.db.QueryRow(ctx, "SELECT COUNT(*) FROM applications").Scan(&n); err != nil || n < 20_000 {
		t.Errorf("rows after rejected write: n=%d err=%v", n, err)
	}
}

func TestWithNoNestLoop_NoCachedPlanCarriesOverToTheSameConnection(t *testing.T) {
	pool, _ := seedClusterFixture(t)
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()
	store := NewPostgresStore(conn)

	q := fixtureZoneQuery("")
	for i := range 8 {
		if _, err := store.FindClustersInZone(ctx, q); err != nil {
			t.Fatalf("FindClustersInZone #%d: %v", i, err)
		}
	}

	query, args := zoneQueryAndArgs(q)
	outside := planShape(parsePlan(t, explainJSON(t, conn, query, args...)))

	fresh, err := pgx.Connect(ctx, pool.Config().ConnString())
	if err != nil {
		t.Fatalf("connect fresh: %v", err)
	}
	defer func() { _ = fresh.Close(ctx) }()
	baseline := planShape(parsePlan(t, explainJSON(t, fresh, query, append([]any{pgx.QueryExecModeExec}, args...)...)))

	if outside != baseline {
		t.Errorf("plan outside the helper differs from a fresh enable_nestloop=on planner:\noutside:  %s\nbaseline: %s", outside, baseline)
	}
}

func TestWithNoNestLoop_ReleasesConnection(t *testing.T) {
	pool, store := seedClusterFixture(t)
	baseline := pool.Stat().AcquiredConns()

	assertReleased := func(t *testing.T) {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for pool.Stat().AcquiredConns() != baseline && time.Now().Before(deadline) {
			time.Sleep(10 * time.Millisecond)
		}
		if got := pool.Stat().AcquiredConns(); got != baseline {
			t.Fatalf("acquired connections: got %d, want %d (leaked)", got, baseline)
		}
	}

	t.Run("callback error", func(t *testing.T) {
		sentinel := errors.New("boom")
		err := store.withNoNestLoop(context.Background(), func(pgx.Tx) error { return sentinel })
		if !errors.Is(err, sentinel) {
			t.Fatalf("got %v, want sentinel", err)
		}
		assertReleased(t)
	})

	t.Run("sql error", func(t *testing.T) {
		ctx := context.Background()
		err := store.withNoNestLoop(ctx, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "SELECT 1/0")
			return err
		})
		if err == nil {
			t.Fatal("want error")
		}
		assertReleased(t)
	})

	t.Run("context already cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := store.withNoNestLoop(ctx, func(pgx.Tx) error { return nil }); err == nil {
			t.Fatal("want error")
		}
		assertReleased(t)
	})

	t.Run("context cancelled mid-flight", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := store.withNoNestLoop(ctx, func(tx pgx.Tx) error {
			cancel()
			_, err := tx.Exec(ctx, "SELECT pg_sleep(5)")
			return err
		})
		if err == nil {
			t.Fatal("want error")
		}
		assertReleased(t)
	})
}
