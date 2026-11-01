//go:build integration

package applications

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// The B cell of the shared fixture: 60 coincident rows, so every page size below
// splits a run of equal distances across a page edge.
const (
	knnCentreLon = 0.0712
	knnCentreLat = 51.60
	knnRadius    = 5_000.0
)

func knnBoundary() []Coordinate {
	const lon, lat = 0.15, 0.06
	return []Coordinate{
		{Longitude: knnCentreLon + lon, Latitude: knnCentreLat + lat},
		{Longitude: knnCentreLon - lon, Latitude: knnCentreLat + lat},
		{Longitude: knnCentreLon - lon, Latitude: knnCentreLat - lat},
		{Longitude: knnCentreLon + lon, Latitude: knnCentreLat - lat},
		{Longitude: knnCentreLon + lon, Latitude: knnCentreLat + lat},
	}
}

func TestWithKNNWalk_SetsBitmapScanOffInsideAndRestoresOnSameConnection(t *testing.T) {
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
		if err := db.QueryRow(ctx, "SHOW enable_bitmapscan").Scan(&v); err != nil {
			t.Fatalf("SHOW enable_bitmapscan: %v", err)
		}
		return v
	}
	if got := show(conn); got != "on" {
		t.Fatalf("before: enable_bitmapscan = %q, want on", got)
	}
	var inside string
	if err := store.withKNNWalk(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SHOW enable_bitmapscan").Scan(&inside)
	}); err != nil {
		t.Fatalf("withKNNWalk: %v", err)
	}
	if inside != "off" {
		t.Errorf("inside: enable_bitmapscan = %q, want off", inside)
	}
	if got := show(conn); got != "on" {
		t.Errorf("after: enable_bitmapscan = %q, want on on the same connection", got)
	}
}

func TestWithKNNWalk_RejectsWrites(t *testing.T) {
	_, store := seedClusterFixture(t)
	ctx := context.Background()
	err := store.withKNNWalk(ctx, func(tx pgx.Tx) error {
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

func TestWithKNNWalk_ReleasesConnection(t *testing.T) {
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
		err := store.withKNNWalk(context.Background(), func(pgx.Tx) error { return sentinel })
		if !errors.Is(err, sentinel) {
			t.Fatalf("got %v, want sentinel", err)
		}
		assertReleased(t)
	})

	t.Run("sql error", func(t *testing.T) {
		ctx := context.Background()
		err := store.withKNNWalk(ctx, func(tx pgx.Tx) error {
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
		if err := store.withKNNWalk(ctx, func(pgx.Tx) error { return nil }); err == nil {
			t.Fatal("want error")
		}
		assertReleased(t)
	})

	t.Run("context cancelled mid-flight", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		err := store.withKNNWalk(ctx, func(tx pgx.Tx) error {
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

// planDescribes flattens an EXPLAIN (FORMAT JSON) document into its node types
// and the Index Scan nodes that carry an Order By clause.
func planDescribes(t *testing.T, raw []byte) (nodeTypes []string, orderedIndexScans int) {
	t.Helper()
	var doc []map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc) != 1 {
		t.Fatalf("parse EXPLAIN JSON (len %d): %v\n%s", len(doc), err, raw)
	}
	var walk func(node map[string]any)
	walk = func(node map[string]any) {
		nt, _ := node["Node Type"].(string)
		nodeTypes = append(nodeTypes, nt)
		if _, ok := node["Order By"]; ok && nt == "Index Scan" {
			orderedIndexScans++
		}
		children, _ := node["Plans"].([]any)
		for _, c := range children {
			if m, ok := c.(map[string]any); ok {
				walk(m)
			}
		}
	}
	root, _ := doc[0]["Plan"].(map[string]any)
	walk(root)
	return nodeTypes, orderedIndexScans
}

func TestKNNWalk_FirstPagePlansWalkTheIndexWithoutBitmapScan(t *testing.T) {
	_, store := seedClusterFixture(t)
	ctx := context.Background()

	const planLon, planLat, planHalf = -0.1, 51.65, 0.01
	boundaryJSON, err := encodeBoundaryGeoJSON([]Coordinate{
		{Longitude: planLon + planHalf, Latitude: planLat + planHalf},
		{Longitude: planLon - planHalf, Latitude: planLat + planHalf},
		{Longitude: planLon - planHalf, Latitude: planLat - planHalf},
		{Longitude: planLon + planHalf, Latitude: planLat - planHalf},
		{Longitude: planLon + planHalf, Latitude: planLat + planHalf},
	})
	if err != nil {
		t.Fatalf("encode boundary: %v", err)
	}
	cases := map[string]struct {
		query string
		args  []any
	}{
		"nearby":   {findNearbyFirstPageQuery, []any{planLon, planLat, 1_500.0, 500}},
		"boundary": {findBoundaryFirstPageQuery, []any{planLon, planLat, boundaryJSON, 500}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var raw []byte
			err := store.withKNNWalk(ctx, func(tx pgx.Tx) error {
				rows, err := tx.Query(ctx, "EXPLAIN (FORMAT JSON) "+tc.query, append([]any{pgx.QueryExecModeExec}, tc.args...)...)
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
				t.Fatalf("explain through withKNNWalk: %v", err)
			}
			nodes, ordered := planDescribes(t, raw)
			if slices.Contains(nodes, "Bitmap Heap Scan") {
				t.Errorf("plan has a Bitmap Heap Scan:\n%s", raw)
			}
			if ordered == 0 {
				t.Errorf("plan has no GiST Index Scan with Order By:\n%s", raw)
			}
		})
	}
}

type knnPageFunc func(ctx context.Context, cursor string) ([]PlanningApplication, string, error)

func pageAll(t *testing.T, fetch knnPageFunc) (names, uids []string, pages int) {
	t.Helper()
	cursor := ""
	for {
		apps, next, err := fetch(context.Background(), cursor)
		if err != nil {
			t.Fatalf("page %d: %v", pages+1, err)
		}
		pages++
		for _, a := range apps {
			names = append(names, a.Name)
			uids = append(uids, a.UID)
		}
		if next == "" {
			return names, uids, pages
		}
		cursor = next
		if pages > 400 {
			t.Fatal("paging did not terminate")
		}
	}
}

func oracle(t *testing.T, store *PostgresStore, filter string, args ...any) (names, uids []string) {
	t.Helper()
	rows, err := store.db.Query(context.Background(),
		"SELECT planit_name, uid FROM applications WHERE "+filter+
			" ORDER BY location <-> "+nearbyPoint+", planit_name, uid", args...)
	if err != nil {
		t.Fatalf("oracle query: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var n, u string
		if err := rows.Scan(&n, &u); err != nil {
			t.Fatalf("oracle scan: %v", err)
		}
		names = append(names, n)
		uids = append(uids, u)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("oracle rows: %v", err)
	}
	return names, uids
}

func assertSameRows(t *testing.T, gotNames, gotUIDs, wantNames, wantUIDs []string) {
	t.Helper()
	if len(gotNames) != len(wantNames) {
		t.Fatalf("row count: got %d, want %d", len(gotNames), len(wantNames))
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		for i := range wantNames {
			if gotNames[i] != wantNames[i] {
				t.Fatalf("order differs at row %d: got %q, want %q", i, gotNames[i], wantNames[i])
			}
		}
	}
	got := slices.Clone(gotUIDs)
	want := slices.Clone(wantUIDs)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("row set differs (gaps or overlaps across pages)")
	}
}

func TestKNNWalk_PagesEqualExhaustiveSort(t *testing.T) {
	_, store := seedClusterFixture(t)
	boundaryJSON, err := encodeBoundaryGeoJSON(knnBoundary())
	if err != nil {
		t.Fatalf("encode boundary: %v", err)
	}

	t.Run("circle", func(t *testing.T) {
		wantNames, wantUIDs := oracle(t, store,
			"ST_DWithin(location, "+nearbyPoint+", $3)", knnCentreLon, knnCentreLat, knnRadius)
		if len(wantNames) < 500 {
			t.Fatalf("fixture too small for the circle: %d rows", len(wantNames))
		}
		for _, limit := range []int{25, 40} {
			names, uids, pages := pageAll(t, func(ctx context.Context, cursor string) ([]PlanningApplication, string, error) {
				return store.FindNearbyPage(ctx, knnCentreLat, knnCentreLon, knnRadius, limit, cursor)
			})
			if pages < 3 {
				t.Fatalf("limit %d: only %d pages", limit, pages)
			}
			assertSameRows(t, names, uids, wantNames, wantUIDs)
		}
	})

	t.Run("boundary", func(t *testing.T) {
		wantNames, wantUIDs := oracle(t, store,
			"ST_Covers("+boundaryPolygon+", location)", knnCentreLon, knnCentreLat, boundaryJSON)
		if len(wantNames) < 500 {
			t.Fatalf("fixture too small for the boundary: %d rows", len(wantNames))
		}
		for _, limit := range []int{25, 100} {
			names, uids, pages := pageAll(t, func(ctx context.Context, cursor string) ([]PlanningApplication, string, error) {
				return store.FindInBoundaryPage(ctx, knnCentreLat, knnCentreLon, knnBoundary(), limit, cursor)
			})
			if pages < 3 {
				t.Fatalf("limit %d: only %d pages", limit, pages)
			}
			assertSameRows(t, names, uids, wantNames, wantUIDs)
		}
	})
}

// oldBoundaryFirstPageQuery is the boundary first-page query before the KNN walk
// change; it issues the cursors a client may still hold.
const oldBoundaryFirstPageQuery = "SELECT " + appColumns + ", location <-> " + nearbyPoint + " AS distance " +
	"FROM applications WHERE ST_Covers(" + boundaryPolygon + ", location) " +
	"ORDER BY location <-> " + nearbyPoint + ", planit_name LIMIT $4"

func TestKNNWalk_CursorFromOldBoundaryQueryResumesOnNewQuery(t *testing.T) {
	_, store := seedClusterFixture(t)
	ctx := context.Background()
	boundaryJSON, err := encodeBoundaryGeoJSON(knnBoundary())
	if err != nil {
		t.Fatalf("encode boundary: %v", err)
	}
	const limit = 30

	rows, err := store.db.Query(ctx, oldBoundaryFirstPageQuery, pgx.QueryExecModeExec, knnCentreLon, knnCentreLat, boundaryJSON, limit)
	if err != nil {
		t.Fatalf("old first page: %v", err)
	}
	old, err := pgx.CollectRows(rows, scanNearbyRow)
	if err != nil || len(old) != limit {
		t.Fatalf("collect old first page: n=%d err=%v", len(old), err)
	}
	last := old[len(old)-1]
	cursor, err := encodeNearbyCursor(last.dist, last.app.Name)
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}

	names, uids, _ := pageAll(t, func(ctx context.Context, c string) ([]PlanningApplication, string, error) {
		if c == "" {
			c = cursor
		}
		return store.FindInBoundaryPage(ctx, knnCentreLat, knnCentreLon, knnBoundary(), limit, c)
	})

	wantNames, wantUIDs := oracle(t, store,
		"ST_Covers("+boundaryPolygon+", location)", knnCentreLon, knnCentreLat, boundaryJSON)
	oldNames := make([]string, 0, len(old))
	for _, o := range old {
		oldNames = append(oldNames, o.app.Name)
	}
	if !strings.HasPrefix(strings.Join(wantNames, ","), strings.Join(oldNames, ",")) {
		t.Fatalf("old first page is not a prefix of the exhaustive order")
	}
	assertSameRows(t, names, uids, wantNames[limit:], wantUIDs[limit:])
}
