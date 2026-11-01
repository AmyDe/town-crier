//go:build integration

package applications

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedThinClusterFixture seeds the shared cluster fixture and copies its located
// rows into application_locations.
func seedThinClusterFixture(t *testing.T) (*pgxpool.Pool, *PostgresStore, *PostgresStore) {
	t.Helper()
	pool, off := seedClusterFixture(t)
	ctx := context.Background()
	for _, s := range []string{
		`TRUNCATE application_locations`,
		`INSERT INTO application_locations (authority_code, planit_name, uid, area_id, location, app_state, start_date)
		SELECT authority_code, planit_name, uid, area_id, location, app_state, start_date
		FROM applications WHERE location IS NOT NULL ORDER BY ST_GeoHash(location::geometry, 10)`,
		`ANALYZE application_locations`,
	} {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("seed application_locations: %v\n%s", err, s)
		}
	}
	return pool, off, NewPostgresStore(pool).WithReadPath(fixedReadPath{mapOn: true})
}

func TestThinMap_ClusterResultsMatchSwitchOff(t *testing.T) {
	pool, off, on := seedThinClusterFixture(t)
	ctx := context.Background()

	for _, status := range []string{"", "Permitted", "Rejected", "Undecided"} {
		t.Run(fmt.Sprintf("zone/status=%q", status), func(t *testing.T) {
			q := fixtureZoneQuery(status)
			want, err := off.FindClustersInZone(ctx, q)
			if err != nil {
				t.Fatalf("off: %v", err)
			}
			got, err := on.FindClustersInZone(ctx, q)
			if err != nil {
				t.Fatalf("on: %v", err)
			}
			if len(want) == 0 {
				t.Fatal("empty result, comparison would be vacuous")
			}
			assertClustersEqual(t, got, want)
			if status == "" {
				assertFixtureCoverage(ctx, t, pool, got)
			}
		})
		t.Run(fmt.Sprintf("boundary/status=%q", status), func(t *testing.T) {
			q := fixtureBoundaryQuery(status)
			want, err := off.FindClustersInBoundary(ctx, q)
			if err != nil {
				t.Fatalf("off: %v", err)
			}
			got, err := on.FindClustersInBoundary(ctx, q)
			if err != nil {
				t.Fatalf("on: %v", err)
			}
			if len(want) == 0 {
				t.Fatal("empty result, comparison would be vacuous")
			}
			assertClustersEqual(t, got, want)
			if status == "" {
				assertFixtureCoverage(ctx, t, pool, got)
			}
		})
	}
}

type clusterStmt struct {
	name string
	run  func(ctx context.Context, s *PostgresStore) error
}

func thinClusterStmts() []clusterStmt {
	var out []clusterStmt
	for _, status := range []string{"", "Permitted"} {
		out = append(out,
			clusterStmt{fmt.Sprintf("zone/status=%q", status), func(ctx context.Context, s *PostgresStore) error {
				_, err := s.FindClustersInZone(ctx, fixtureZoneQuery(status))
				return err
			}},
			clusterStmt{fmt.Sprintf("boundary/status=%q", status), func(ctx context.Context, s *PostgresStore) error {
				_, err := s.FindClustersInBoundary(ctx, fixtureBoundaryQuery(status))
				return err
			}})
	}
	return out
}

func isClusterStmt(s recordedStmt) bool { return strings.Contains(s.sql, "unsplittable AS") }

func recordClusterStmt(t *testing.T, c clusterStmt, s *PostgresStore, rec *sqlRecorder) (stmts []recordedStmt, idx int) {
	t.Helper()
	rec.drain()
	if err := c.run(context.Background(), s); err != nil {
		t.Fatalf("%s: %v", c.name, err)
	}
	stmts = rec.drain()
	for i, st := range stmts {
		if isClusterStmt(st) {
			return stmts, i
		}
	}
	t.Fatalf("%s: no cluster statement recorded", c.name)
	return nil, 0
}

func TestThinMap_ClusterRunsInExecModeAfterSetLocalNestLoopOff(t *testing.T) {
	pool, _, _ := seedThinClusterFixture(t)
	on, rec := recordedStore(t, pool, fixedReadPath{mapOn: true})

	for _, c := range thinClusterStmts() {
		stmts, i := recordClusterStmt(t, c, on, rec)
		st := stmts[i]
		if !strings.Contains(st.sql, "FROM "+thinTable+"\n") {
			t.Errorf("%s: cluster statement does not read %s", c.name, thinTable)
		}
		if len(st.args) == 0 || st.args[0] != pgx.QueryExecModeExec {
			t.Errorf("%s: cluster statement does not use QueryExecModeExec", c.name)
		}
		if i == 0 || stmts[i-1].sql != "SET LOCAL enable_nestloop = off" {
			t.Errorf("%s: cluster statement does not run right after SET LOCAL enable_nestloop = off", c.name)
		}
	}
}

type analyzeNode struct {
	NodeType     string        `json:"Node Type"`
	RelationName string        `json:"Relation Name"`
	IndexName    string        `json:"Index Name"`
	ActualRows   float64       `json:"Actual Rows"`
	ActualLoops  float64       `json:"Actual Loops"`
	Plans        []analyzeNode `json:"Plans"`
}

func (n analyzeNode) rows() float64 { return n.ActualRows * n.ActualLoops }

func (n analyzeNode) flatten(out *[]analyzeNode) {
	*out = append(*out, n)
	for _, c := range n.Plans {
		c.flatten(out)
	}
}

// explainAnalyze runs EXPLAIN (ANALYZE, FORMAT JSON) of a recorded cluster
// statement on one connection with the given session settings.
func explainAnalyze(t *testing.T, pool *pgxpool.Pool, settings []string, st recordedStmt) analyzeNode {
	t.Helper()
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer conn.Release()
	for _, set := range settings {
		if _, err := conn.Exec(ctx, set); err != nil {
			t.Fatalf("%s: %v", set, err)
		}
	}
	rows, err := conn.Query(ctx, "EXPLAIN (ANALYZE, FORMAT JSON) "+st.sql, st.args...)
	if err != nil {
		t.Fatalf("EXPLAIN ANALYZE: %v", err)
	}
	raw, err := pgx.CollectOneRow(rows, func(r pgx.CollectableRow) ([]byte, error) {
		var b []byte
		err := r.Scan(&b)
		return b, err
	})
	if err != nil {
		t.Fatalf("read EXPLAIN: %v", err)
	}
	var doc []struct {
		Plan analyzeNode `json:"Plan"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc) != 1 {
		t.Fatalf("parse EXPLAIN (len %d): %v\n%s", len(doc), err, raw)
	}
	return doc[0].Plan
}

// TestThinMap_ClusterPlansHaveNoNestedLoopAndNeverReadApplications plans the
// statement with enable_nestloop = off, as the store runs it. Unforced, the
// planner picks a nested loop for three of the four shapes on this fixture; the
// unfiltered boundary shape plans without one either way.
func TestThinMap_ClusterPlansHaveNoNestedLoopAndNeverReadApplications(t *testing.T) {
	pool, _, _ := seedThinClusterFixture(t)
	on, rec := recordedStore(t, pool, fixedReadPath{mapOn: true})

	for _, c := range thinClusterStmts() {
		stmts, i := recordClusterStmt(t, c, on, rec)
		var nodes []analyzeNode
		explainAnalyze(t, pool, []string{"SET enable_nestloop = off"}, stmts[i]).flatten(&nodes)
		var reads int
		for _, n := range nodes {
			if n.NodeType == "Nested Loop" {
				t.Errorf("%s: plan has a Nested Loop", c.name)
			}
			if n.RelationName == "applications" {
				t.Errorf("%s: plan reads applications (%s)", c.name, n.NodeType)
			}
			if n.RelationName == thinTable {
				reads++
			}
		}
		if reads == 0 {
			t.Errorf("%s: plan does not read %s", c.name, thinTable)
		}
	}
}

// TestThinMap_CappedViewportUsesTheGeometryGiSTIndex asks for a small viewport
// inside a large zone; the planner picks the index with no settings forced.
func TestThinMap_CappedViewportUsesTheGeometryGiSTIndex(t *testing.T) {
	pool, _, _ := seedThinClusterFixture(t)
	on, rec := recordedStore(t, pool, fixedReadPath{mapOn: true})

	zone := fixtureZoneQuery("")
	zone.West, zone.South, zone.East, zone.North = -0.05, 51.55, 0.0, 51.6
	boundary := fixtureBoundaryQuery("")
	boundary.West, boundary.South, boundary.East, boundary.North = -0.05, 51.55, 0.0, 51.6
	cases := []clusterStmt{
		{"zone", func(ctx context.Context, s *PostgresStore) error {
			_, err := s.FindClustersInZone(ctx, zone)
			return err
		}},
		{"boundary", func(ctx context.Context, s *PostgresStore) error {
			_, err := s.FindClustersInBoundary(ctx, boundary)
			return err
		}},
	}
	for _, c := range cases {
		stmts, i := recordClusterStmt(t, c, on, rec)
		var nodes []analyzeNode
		explainAnalyze(t, pool, []string{"SET enable_nestloop = off"}, stmts[i]).flatten(&nodes)
		var used bool
		for _, n := range nodes {
			if n.IndexName == "application_locations_geometry" {
				used = true
			}
		}
		if !used {
			t.Errorf("%s: plan does not use application_locations_geometry", c.name)
		}
	}
}

// TestThinMap_RankSortReadsOnlyUnsplittableCells checks the window that ranks
// members is fed only the rows of unsplittable cells, on a fixture where most
// rows sit in splittable cells.
func TestThinMap_RankSortReadsOnlyUnsplittableCells(t *testing.T) {
	pool, _, _ := seedThinClusterFixture(t)
	on, rec := recordedStore(t, pool, fixedReadPath{mapOn: true})
	ctx := context.Background()

	for _, c := range thinClusterStmts() {
		if !strings.HasSuffix(c.name, `status=""`) {
			continue
		}
		var unsplittableRows, splittableRows int
		var clusters []Cluster
		var err error
		if strings.HasPrefix(c.name, "zone") {
			clusters, err = on.FindClustersInZone(ctx, fixtureZoneQuery(""))
		} else {
			clusters, err = on.FindClustersInBoundary(ctx, fixtureBoundaryQuery(""))
		}
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for _, cl := range clusters {
			if cl.Members != nil {
				unsplittableRows += cl.Count
			} else {
				splittableRows += cl.Count
			}
		}
		if unsplittableRows == 0 || splittableRows < 10*unsplittableRows {
			t.Fatalf("%s fixture: %d unsplittable rows, %d splittable; want a small unsplittable share", c.name, unsplittableRows, splittableRows)
		}

		stmts, i := recordClusterStmt(t, c, on, rec)
		var nodes []analyzeNode
		explainAnalyze(t, pool, []string{"SET enable_nestloop = off"}, stmts[i]).flatten(&nodes)
		var windows int
		for _, n := range nodes {
			if n.NodeType != "WindowAgg" {
				continue
			}
			windows++
			if len(n.Plans) != 1 {
				t.Fatalf("%s: WindowAgg has %d children, want 1", c.name, len(n.Plans))
			}
			if got := n.Plans[0].rows(); got != float64(unsplittableRows) {
				t.Errorf("%s: WindowAgg input (%s) read %v rows, want the %d rows of unsplittable cells",
					c.name, n.Plans[0].NodeType, got, unsplittableRows)
			}
		}
		if windows != 1 {
			t.Errorf("%s: plan has %d WindowAgg nodes, want 1", c.name, windows)
		}
	}
}
