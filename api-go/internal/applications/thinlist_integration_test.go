//go:build integration

package applications

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

const (
	thinUser   = "user-thin"
	thinRadius = 5000.0
	thinLimit  = 5
)

type fixedReadPath struct{ list, mapOn bool }

func (f fixedReadPath) ThinList(context.Context) bool { return f.list }
func (f fixedReadPath) ThinMap(context.Context) bool  { return f.mapOn }

type thinCase struct {
	name string
	page func(ctx context.Context, s *PostgresStore, cursor string) ([]PlanningApplication, string, error)
}

// thinCases is the one table every switch-on against switch-off test shares: each
// sort and filter for the circle zone and the boundary zone, plus the two
// distance-only reads.
func thinCases() []thinCase {
	sorts := []Sort{SortDistance, SortNewest, SortOldest, SortStatus, SortRecentActivity}
	filters := []struct {
		name   string
		status string
		unread bool
	}{{"none", "", false}, {"status", "Undecided", false}, {"unread", "", true}}

	var cases []thinCase
	for _, srt := range sorts {
		for _, f := range filters {
			cases = append(cases,
				thinCase{
					name: fmt.Sprintf("zone/%s/%s", srt, f.name),
					page: func(ctx context.Context, s *PostgresStore, cursor string) ([]PlanningApplication, string, error) {
						return s.FindInZonePage(ctx, InZoneQuery{
							UserID: thinUser, Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: thinRadius,
							Sort: srt, Status: f.status, Unread: f.unread, Limit: thinLimit, Cursor: cursor,
						})
					},
				},
				thinCase{
					name: fmt.Sprintf("boundary-zone/%s/%s", srt, f.name),
					page: func(ctx context.Context, s *PostgresStore, cursor string) ([]PlanningApplication, string, error) {
						return s.FindInBoundaryZonePage(ctx, InBoundaryQuery{
							UserID: thinUser, Latitude: pgCentreLat, Longitude: pgCentreLon, Boundary: squareBoundary(0.05),
							Sort: srt, Status: f.status, Unread: f.unread, Limit: thinLimit, Cursor: cursor,
						})
					},
				})
		}
	}
	cases = append(cases,
		thinCase{
			name: "boundary",
			page: func(ctx context.Context, s *PostgresStore, cursor string) ([]PlanningApplication, string, error) {
				return s.FindInBoundaryPage(ctx, pgCentreLat, pgCentreLon, squareBoundary(0.05), thinLimit, cursor)
			},
		},
		thinCase{
			name: "nearby",
			page: func(ctx context.Context, s *PostgresStore, cursor string) ([]PlanningApplication, string, error) {
				return s.FindNearbyPage(ctx, pgCentreLat, pgCentreLon, thinRadius, thinLimit, cursor)
			},
		})
	return cases
}

// seedThinFixture replaces the data with 70 near applications over two
// authorities (ties on start_date, app_state and distance, NULL start_date and
// app_state), a same-named pair in both authorities, a NULL-location row and
// four far-away rows, copies the located ones into application_locations and
// gives 18 of them an unread notification.
func seedThinFixture(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	pgtest.Truncate(t, pool, "applications", "application_locations", "notifications", "notification_state", "read_path_control", "watch_zones")

	stmts := []string{
		`INSERT INTO applications (planit_name, authority_code, uid, area_name, area_id, app_state, start_date, location, last_different)
		SELECT 'P' || lpad(i::text, 3, '0'), CASE WHEN i % 2 = 0 THEN '100' ELSE '200' END, 'uid-' || i, 'Test',
			CASE WHEN i % 2 = 0 THEN 100 ELSE 200 END,
			(ARRAY['Undecided', 'Permitted', 'Rejected', NULL])[1 + i % 4],
			CASE WHEN i % 7 = 0 THEN NULL ELSE DATE '2026-01-01' + (i % 5) END,
			CASE
				WHEN i > 66 THEN ST_SetSRID(ST_MakePoint(0.8722, 51.5074), 4326)::geography
				WHEN i % 10 = 0 THEN ST_SetSRID(ST_MakePoint(-0.1258, 51.5094), 4326)::geography
				ELSE ST_SetSRID(ST_MakePoint(-0.1278 + ((i * 37) % 11 - 5) * 0.004, 51.5074 + ((i * 53) % 13 - 6) * 0.003), 4326)::geography
			END, now()
		FROM generate_series(1, 70) i`,
		`INSERT INTO applications (planit_name, authority_code, uid, area_name, area_id, app_state, start_date, location, last_different)
		SELECT 'DUP/1', a, 'uid-dup-' || a, 'Test', a::int, 'Undecided', DATE '2026-01-03',
			ST_SetSRID(ST_MakePoint(-0.1268 + a::int * 0.00001, 51.5084), 4326)::geography, now()
		FROM unnest(ARRAY['100', '200']) a`,
		`INSERT INTO applications (planit_name, authority_code, uid, area_name, area_id, app_state, start_date, last_different)
		VALUES ('NOLOC', '100', 'uid-noloc', 'Test', 100, 'Undecided', DATE '2026-01-02', now())`,
		`INSERT INTO application_locations (authority_code, planit_name, uid, area_id, location, app_state, start_date)
		SELECT authority_code, planit_name, uid, area_id, location, app_state, start_date
		FROM applications WHERE location IS NOT NULL ORDER BY ST_GeoHash(location::geometry, 10)`,
		`INSERT INTO notifications (id, user_id, application_uid, authority_id, event_type, created_at)
		SELECT 'n-' || i, '` + thinUser + `', 'uid-' || i, CASE WHEN i % 2 = 0 THEN 100 ELSE 200 END, 'NewApplication',
			TIMESTAMPTZ '2026-01-03 00:00:00+00' + i * INTERVAL '1 hour'
		FROM generate_series(1, 70) i WHERE i % 4 = 1`,
		`ANALYZE applications`,
		`ANALYZE application_locations`,
		`ANALYZE notifications`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("seed thin fixture: %v\n%s", err, s)
		}
	}
}

type thinPage struct {
	apps   []PlanningApplication
	cursor string
}

// pagesWith pages one case to exhaustion, taking each page from pick(pageIndex).
func pagesWith(t *testing.T, c thinCase, pick func(i int) *PostgresStore) []thinPage {
	t.Helper()
	ctx := context.Background()
	var pages []thinPage
	cursor := ""
	for i := range 200 {
		apps, next, err := c.page(ctx, pick(i), cursor)
		if err != nil {
			t.Fatalf("%s page %d (cursor %q): %v", c.name, i, cursor, err)
		}
		pages = append(pages, thinPage{apps: apps, cursor: next})
		if next == "" {
			return pages
		}
		cursor = next
	}
	t.Fatalf("%s did not exhaust within the safety bound", c.name)
	return nil
}

func assertPagesEqual(t *testing.T, name string, got, want []thinPage) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d pages, want %d", name, len(got), len(want))
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Fatalf("%s page %d:\n got  %v cursor %q\n want %v cursor %q", name, i,
				appNames(got[i].apps), got[i].cursor, appNames(want[i].apps), want[i].cursor)
		}
	}
}

func thinStores(pool *pgxpool.Pool) (off, on *PostgresStore) {
	return NewPostgresStore(pool), NewPostgresStore(pool).WithReadPath(fixedReadPath{list: true})
}

func TestThinList_SwitchOnMatchesOffAcrossSortsFiltersAndShapes(t *testing.T) {
	pool := pgtest.New(t)
	seedThinFixture(t, pool)
	off, on := thinStores(pool)

	for _, c := range thinCases() {
		want := pagesWith(t, c, func(int) *PostgresStore { return off })
		if len(want) < 3 {
			t.Fatalf("%s: fixture gives %d pages, want at least 3", c.name, len(want))
		}
		got := pagesWith(t, c, func(int) *PostgresStore { return on })
		assertPagesEqual(t, c.name, got, want)
	}
}

func TestThinList_CursorsWorkAcrossTheSwitch(t *testing.T) {
	pool := pgtest.New(t)
	seedThinFixture(t, pool)
	off, on := thinStores(pool)

	for _, c := range thinCases() {
		want := pagesWith(t, c, func(int) *PostgresStore { return off })
		offFirst := pagesWith(t, c, func(i int) *PostgresStore {
			if i%2 == 0 {
				return off
			}
			return on
		})
		assertPagesEqual(t, c.name+" off then on", offFirst, want)
		onFirst := pagesWith(t, c, func(i int) *PostgresStore {
			if i%2 == 0 {
				return on
			}
			return off
		})
		assertPagesEqual(t, c.name+" on then off", onFirst, want)
	}
}

func TestThinList_RowDeletedBetweenFindAndHydrateIsDropped(t *testing.T) {
	pool := pgtest.New(t)
	off, on := thinStores(pool)
	ctx := context.Background()

	for _, c := range thinCases() {
		for _, victim := range []int{0, thinLimit - 1} {
			seedThinFixture(t, pool)
			want := pagesWith(t, c, func(int) *PostgresStore { return off })
			gone := want[0].apps[victim]

			if _, err := pool.Exec(ctx, "DELETE FROM applications WHERE planit_name = $1 AND area_id = $2", gone.Name, gone.AreaID); err != nil {
				t.Fatalf("delete %s: %v", gone.Name, err)
			}

			got := pagesWith(t, c, func(int) *PostgresStore { return on })
			name := fmt.Sprintf("%s without row %d", c.name, victim)
			if len(got) != len(want) {
				t.Fatalf("%s: %d pages, want %d", name, len(got), len(want))
			}
			wantFirst := append(append([]PlanningApplication{}, want[0].apps[:victim]...), want[0].apps[victim+1:]...)
			if !reflect.DeepEqual(got[0].apps, wantFirst) {
				t.Fatalf("%s page 0: got %v, want %v", name, appNames(got[0].apps), appNames(wantFirst))
			}
			if got[0].cursor != want[0].cursor {
				t.Fatalf("%s page 0 cursor %q, want %q", name, got[0].cursor, want[0].cursor)
			}
			assertPagesEqual(t, name, got[1:], want[1:])
		}
	}
}

type recordedStmt struct {
	sql  string
	args []any
}

type sqlRecorder struct {
	mu    sync.Mutex
	stmts []recordedStmt
}

func (r *sqlRecorder) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stmts = append(r.stmts, recordedStmt{sql: d.SQL, args: d.Args})
	return ctx
}

func (*sqlRecorder) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (r *sqlRecorder) drain() []recordedStmt {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.stmts
	r.stmts = nil
	return out
}

// recordedThinStore returns a thin-list store on its own pool that records every
// statement it runs.
func recordedThinStore(t *testing.T, pool *pgxpool.Pool) (*PostgresStore, *sqlRecorder) {
	t.Helper()
	rec := &sqlRecorder{}
	cfg := pool.Config().Copy()
	cfg.ConnConfig.Tracer = rec
	traced, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("traced pool: %v", err)
	}
	t.Cleanup(traced.Close)
	return NewPostgresStore(traced).WithReadPath(fixedReadPath{list: true}), rec
}

type explainNode struct {
	NodeType     string        `json:"Node Type"`
	RelationName string        `json:"Relation Name"`
	IndexName    string        `json:"Index Name"`
	Plans        []explainNode `json:"Plans"`
}

func (n explainNode) flatten(out *[]explainNode) {
	*out = append(*out, n)
	for _, c := range n.Plans {
		c.flatten(out)
	}
}

// firstTwoPages runs the first page and, when there is one, the second of a case
// on the recording store and returns the statements it issued.
func firstTwoPages(t *testing.T, c thinCase, s *PostgresStore, rec *sqlRecorder) []recordedStmt {
	t.Helper()
	ctx := context.Background()
	rec.drain()
	_, cursor, err := c.page(ctx, s, "")
	if err != nil {
		t.Fatalf("%s: %v", c.name, err)
	}
	if cursor == "" {
		t.Fatalf("%s: first page has no cursor", c.name)
	}
	if _, _, err := c.page(ctx, s, cursor); err != nil {
		t.Fatalf("%s second page: %v", c.name, err)
	}
	return rec.drain()
}

func explainNodes(t *testing.T, pool *pgxpool.Pool, settings []string, s recordedStmt) []explainNode {
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
	raw := explainJSON(t, conn, s.sql, s.args...)
	var doc []struct {
		Plan explainNode `json:"Plan"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc) != 1 {
		t.Fatalf("parse EXPLAIN (len %d): %v\n%s", len(doc), err, raw)
	}
	var nodes []explainNode
	doc[0].Plan.flatten(&nodes)
	return nodes
}

func isFind(s recordedStmt) bool    { return strings.Contains(s.sql, "FROM application_locations") }
func isHydrate(s recordedStmt) bool { return strings.Contains(s.sql, "unnest(") }

func TestThinList_FindPlansNeverReadApplicationsAndHydrateUsesPrimaryKey(t *testing.T) {
	pool := pgtest.New(t)
	seedThinFixture(t, pool)
	on, rec := recordedThinStore(t, pool)

	for _, c := range thinCases() {
		var finds, hydrates int
		for _, s := range firstTwoPages(t, c, on, rec) {
			switch {
			case isFind(s):
				finds++
				for _, n := range explainNodes(t, pool, []string{"SET enable_nestloop = off"}, s) {
					if n.RelationName == "applications" {
						t.Errorf("%s: find plan reads applications (%s)\n%s", c.name, n.NodeType, s.sql)
					}
				}
			case isHydrate(s):
				hydrates++
				var usesPK bool
				for _, n := range explainNodes(t, pool, []string{"SET enable_seqscan = off"}, s) {
					if n.RelationName != "applications" && n.IndexName == "" {
						continue
					}
					if n.NodeType == "Seq Scan" && n.RelationName == "applications" {
						t.Errorf("%s: hydrate plan seq-scans applications\n%s", c.name, s.sql)
					}
					if n.IndexName == "applications_pkey" {
						usesPK = true
					} else if n.IndexName != "" {
						t.Errorf("%s: hydrate plan uses index %s, want applications_pkey only", c.name, n.IndexName)
					}
				}
				if !usesPK {
					t.Errorf("%s: hydrate plan does not use applications_pkey\n%s", c.name, s.sql)
				}
			}
		}
		if finds == 0 || hydrates == 0 {
			t.Errorf("%s: recorded %d find and %d hydrate statements, want both", c.name, finds, hydrates)
		}
	}
}

func TestThinList_EveryFindRunsInExecMode(t *testing.T) {
	pool := pgtest.New(t)
	seedThinFixture(t, pool)
	on, rec := recordedThinStore(t, pool)

	var finds int
	for _, c := range thinCases() {
		for _, s := range firstTwoPages(t, c, on, rec) {
			if !isFind(s) {
				continue
			}
			finds++
			if len(s.args) == 0 || s.args[0] != pgx.QueryExecModeExec {
				t.Errorf("%s: find does not use QueryExecModeExec\n%s", c.name, s.sql)
			}
		}
	}
	if finds == 0 {
		t.Fatal("no find statements recorded")
	}
}

func TestThinList_ActivityAndUnreadFindsRunWithNestedLoopsOffAndPlanWithoutThem(t *testing.T) {
	pool := pgtest.New(t)
	seedThinFixture(t, pool)
	on, rec := recordedThinStore(t, pool)

	var checked int
	for _, c := range thinCases() {
		stmts := firstTwoPages(t, c, on, rec)
		for i, s := range stmts {
			if !isFind(s) || !strings.Contains(s.sql, "notifications") {
				continue
			}
			checked++
			if i == 0 || stmts[i-1].sql != "SET LOCAL enable_nestloop = off" {
				t.Errorf("%s: find does not run right after SET LOCAL enable_nestloop = off\n%s", c.name, s.sql)
			}
			for _, n := range explainNodes(t, pool, []string{"SET enable_nestloop = off"}, s) {
				if n.NodeType == "Nested Loop" {
					t.Errorf("%s: find plan has a Nested Loop\n%s", c.name, s.sql)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no recent-activity or unread find statements recorded")
	}
}
