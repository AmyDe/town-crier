//go:build integration

package applications

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// generousZoneBoundary is a square ring around the shared London centre
// fixture, large enough (a ~0.02-degree half-side, ~2.2 km) to comfortably
// contain every sortFixture/statusFixture/activityFixture/unreadFixture point
// (all offset due north of the centre by at most 800 m), so these tests prove
// ordering, filtering, pagination and cursor semantics -- not containment.
// TestPostgresStore_FindInBoundaryZonePage_CoversPolygonNotEnclosingCircle and
// TestPostgresStore_FindInBoundaryZonePage_RespectsBoundary cover containment
// itself, mirroring boundarypage_integration_test.go's squareBoundary proof.
func generousZoneBoundary() []Coordinate {
	return squareBoundary(0.02)
}

// boundaryZoneQuery is a brief InBoundaryQuery builder fixed to the standard
// generous London fixture boundary, for the cursor/sort-error tests -- the
// boundary-scoped sibling of zoneQuery.
func boundaryZoneQuery(sort Sort, limit int, cursor string) InBoundaryQuery {
	return InBoundaryQuery{
		Latitude: pgCentreLat, Longitude: pgCentreLon, Boundary: generousZoneBoundary(),
		Sort: sort, Limit: limit, Cursor: cursor,
	}
}

// pageAllInBoundaryZone pages FindInBoundaryZonePage to exhaustion as an
// anonymous caller (no userID), for the sorts that ignore per-user data --
// the boundary-scoped sibling of pageAllInZone.
func pageAllInBoundaryZone(t *testing.T, store *PostgresStore, sort Sort, limit int) []string {
	t.Helper()
	return pageAllInBoundaryZoneAs(t, store, "", sort, limit)
}

// pageAllInBoundaryZoneAs pages FindInBoundaryZonePage to exhaustion for the
// given caller and page size, returning the names in the order seen across
// pages -- the boundary-scoped sibling of pageAllInZoneAs.
func pageAllInBoundaryZoneAs(t *testing.T, store *PostgresStore, userID string, sort Sort, limit int) []string {
	t.Helper()
	ctx := context.Background()
	var names []string
	cursor := ""
	for range 1000 {
		page, next, err := store.FindInBoundaryZonePage(ctx, InBoundaryQuery{
			UserID: userID, Latitude: pgCentreLat, Longitude: pgCentreLon,
			Boundary: generousZoneBoundary(), Sort: sort, Limit: limit, Cursor: cursor,
		})
		if err != nil {
			t.Fatalf("FindInBoundaryZonePage(%s, cursor=%q): %v", sort, cursor, err)
		}
		names = append(names, appNames(page)...)
		if next == "" {
			return names
		}
		cursor = next
	}
	t.Fatalf("FindInBoundaryZonePage(%s) did not exhaust within the safety bound", sort)
	return nil
}

// pageAllBoundaryZoneFiltered pages FindInBoundaryZonePage to exhaustion for
// an arbitrary query (sort + status/unread filter + page size) -- the
// boundary-scoped sibling of pageAllFiltered.
func pageAllBoundaryZoneFiltered(t *testing.T, store *PostgresStore, base InBoundaryQuery) []string {
	t.Helper()
	ctx := context.Background()
	var names []string
	cursor := ""
	for range 1000 {
		q := base
		q.Cursor = cursor
		page, next, err := store.FindInBoundaryZonePage(ctx, q)
		if err != nil {
			t.Fatalf("FindInBoundaryZonePage(sort=%s status=%q unread=%t cursor=%q): %v", base.Sort, base.Status, base.Unread, cursor, err)
		}
		names = append(names, appNames(page)...)
		if next == "" {
			return names
		}
		cursor = next
	}
	t.Fatalf("FindInBoundaryZonePage(sort=%s status=%q unread=%t) did not exhaust within the safety bound", base.Sort, base.Status, base.Unread)
	return nil
}

// TestPostgresStore_FindInBoundaryZonePage_Distance proves the boundary-scoped
// sort-aware path reproduces FindInBoundaryPage's nearest-centroid-first
// ordering and pages without gaps.
func TestPostgresStore_FindInBoundaryZonePage_Distance(t *testing.T) {
	store := newAppPGStore(t)
	sortFixture(t, store)

	want := []string{"A1", "A2", "A3", "A4", "A5", "A6"}
	if got := pageAllInBoundaryZone(t, store, SortDistance, 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("distance single page: got %v, want %v", got, want)
	}
	if got := pageAllInBoundaryZone(t, store, SortDistance, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("distance paged: got %v, want %v", got, want)
	}
}

// TestPostgresStore_FindInBoundaryZonePage_Newest proves start_date DESC NULLS
// LAST with the (authority_code, planit_name) tiebreak, paged to exhaustion
// with no overlap or gap, matches the single unpaged order -- identical to
// FindInZonePage's ordering, just polygon-scoped.
func TestPostgresStore_FindInBoundaryZonePage_Newest(t *testing.T) {
	store := newAppPGStore(t)
	sortFixture(t, store)

	want := []string{"A1", "A3", "A4", "A2", "A5", "A6"}
	if got := pageAllInBoundaryZone(t, store, SortNewest, 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("newest single page: got %v, want %v", got, want)
	}
	if got := pageAllInBoundaryZone(t, store, SortNewest, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("newest paged: got %v, want %v", got, want)
	}
	if got := pageAllInBoundaryZone(t, store, SortNewest, 1); !reflect.DeepEqual(got, want) {
		t.Fatalf("newest paged size 1: got %v, want %v", got, want)
	}
}

// TestPostgresStore_FindInBoundaryZonePage_Oldest proves start_date ASC NULLS
// LAST with the same tiebreak, paged to exhaustion, matches the single
// unpaged order.
func TestPostgresStore_FindInBoundaryZonePage_Oldest(t *testing.T) {
	store := newAppPGStore(t)
	sortFixture(t, store)

	want := []string{"A2", "A3", "A4", "A1", "A5", "A6"}
	if got := pageAllInBoundaryZone(t, store, SortOldest, 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("oldest single page: got %v, want %v", got, want)
	}
	if got := pageAllInBoundaryZone(t, store, SortOldest, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("oldest paged: got %v, want %v", got, want)
	}
}

// TestPostgresStore_FindInBoundaryZonePage_Status proves the mixed-direction
// keyset (app_state ASC NULLS LAST, start_date DESC NULLS LAST, tiebreak)
// pages to exhaustion with no overlap or gap, matching FindInZonePage's
// status ordering exactly.
func TestPostgresStore_FindInBoundaryZonePage_Status(t *testing.T) {
	store := newAppPGStore(t)
	statusFixture(t, store)

	want := []string{"P1", "P3", "P4", "P2", "P5", "R1", "R2", "N1", "N2"}
	if got := pageAllInBoundaryZone(t, store, SortStatus, 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("status single page: got %v, want %v", got, want)
	}
	if got := pageAllInBoundaryZone(t, store, SortStatus, 1); !reflect.DeepEqual(got, want) {
		t.Fatalf("status paged size 1: got %v, want %v", got, want)
	}
	if got := pageAllInBoundaryZone(t, store, SortStatus, 3); !reflect.DeepEqual(got, want) {
		t.Fatalf("status paged size 3: got %v, want %v", got, want)
	}
}

// TestPostgresStore_FindInBoundaryZonePage_RecentActivity proves the
// GREATEST(start_date, unread.created_at) DESC NULLS LAST ordering, paged to
// exhaustion, matches the single unpaged order -- the polygon-scoped
// equivalent of FindInZonePage's headline recent-activity behaviour.
func TestPostgresStore_FindInBoundaryZonePage_RecentActivity(t *testing.T) {
	store, pool := newActivityPGStore(t)
	const userID = "user-boundary-activity"
	activityFixture(t, store, pool, userID)

	want := []string{"AA", "BB", "CC", "DD", "EE", "FF"}
	got := pageAllInBoundaryZoneAs(t, store, userID, SortRecentActivity, 100)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recent-activity single page: got %v, want %v", got, want)
	}
	if got := pageAllInBoundaryZoneAs(t, store, userID, SortRecentActivity, 1); !reflect.DeepEqual(got, want) {
		t.Fatalf("recent-activity paged size 1: got %v, want %v", got, want)
	}
}

// TestPostgresStore_FindInBoundaryZonePage_StatusFilter proves ?status=
// restricts the polygon-scoped candidate set to exactly the matching
// app_state, composes with every scalar sort, and pages to exhaustion with no
// overlap or gap.
func TestPostgresStore_FindInBoundaryZonePage_StatusFilter(t *testing.T) {
	store := newAppPGStore(t)
	statusFixture(t, store)

	permittedByDate := []string{"P1", "P3", "P4", "P2", "P5"}
	permittedByDistance := []string{"P1", "P2", "P3", "P4", "P5"}
	cases := []struct {
		sort Sort
		want []string
	}{
		{SortNewest, permittedByDate},
		{SortStatus, permittedByDate},
		{SortDistance, permittedByDistance},
	}
	for _, tc := range cases {
		for _, limit := range []int{100, 2, 1} {
			base := InBoundaryQuery{
				Latitude: pgCentreLat, Longitude: pgCentreLon, Boundary: generousZoneBoundary(),
				Sort: tc.sort, Status: "Permitted", Limit: limit,
			}
			if got := pageAllBoundaryZoneFiltered(t, store, base); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("status=Permitted sort=%s limit=%d: got %v, want %v", tc.sort, limit, got, tc.want)
			}
		}
	}

	empty := pageAllBoundaryZoneFiltered(t, store, InBoundaryQuery{
		Latitude: pgCentreLat, Longitude: pgCentreLon, Boundary: generousZoneBoundary(),
		Sort: SortNewest, Status: "Withdrawn", Limit: 10,
	})
	if len(empty) != 0 {
		t.Fatalf("status=Withdrawn (no rows): got %v, want empty", empty)
	}
}

// TestPostgresStore_FindInBoundaryZonePage_UnreadFilter proves ?unread=true
// (the INNER JOIN to the caller's unread notifications) keeps only
// applications with an unread notification, composes with scalar sorts, and
// pages without gaps -- the polygon-scoped equivalent of FindInZonePage's
// unread filter.
func TestPostgresStore_FindInBoundaryZonePage_UnreadFilter(t *testing.T) {
	store, pool := newActivityPGStore(t)
	const userID = "user-boundary-unread"
	unreadFixture(t, store, pool, userID)

	cases := []struct {
		sort Sort
		want []string
	}{
		{SortNewest, []string{"U4", "U1"}},
		{SortDistance, []string{"U1", "U4"}},
	}
	for _, tc := range cases {
		for _, limit := range []int{100, 1} {
			base := InBoundaryQuery{
				UserID: userID, Latitude: pgCentreLat, Longitude: pgCentreLon, Boundary: generousZoneBoundary(),
				Sort: tc.sort, Unread: true, Limit: limit,
			}
			if got := pageAllBoundaryZoneFiltered(t, store, base); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unread=true sort=%s limit=%d: got %v, want %v", tc.sort, limit, got, tc.want)
			}
		}
	}
}

// TestPostgresStore_FindInBoundaryZonePage_CursorSortMismatch proves a cursor
// minted under one sort is rejected when replayed under another, and a
// malformed cursor is reported as ErrCursorInvalid -- identical semantics to
// FindInZonePage's cursor guard.
func TestPostgresStore_FindInBoundaryZonePage_CursorSortMismatch(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)
	sortFixture(t, store)

	_, cursor, err := store.FindInBoundaryZonePage(ctx, boundaryZoneQuery(SortNewest, 1, ""))
	if err != nil {
		t.Fatalf("mint newest cursor: %v", err)
	}
	if cursor == "" {
		t.Fatal("expected a continuation cursor after a full page")
	}
	if _, _, err := store.FindInBoundaryZonePage(ctx, boundaryZoneQuery(SortOldest, 1, cursor)); !errors.Is(err, ErrCursorSortMismatch) {
		t.Errorf("replay newest cursor under oldest: got err %v, want ErrCursorSortMismatch", err)
	}
	if _, _, err := store.FindInBoundaryZonePage(ctx, boundaryZoneQuery(SortNewest, 1, "!!!not-base64!!!")); !errors.Is(err, ErrCursorInvalid) {
		t.Errorf("malformed cursor: got err %v, want ErrCursorInvalid", err)
	}
}

// TestPostgresStore_FindInBoundaryZonePage_CursorFilterMismatch proves a
// cursor minted under one filter is rejected when replayed under another --
// identical semantics to FindInZonePage's filter guard.
func TestPostgresStore_FindInBoundaryZonePage_CursorFilterMismatch(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)
	statusFixture(t, store)

	permitted := InBoundaryQuery{
		Latitude: pgCentreLat, Longitude: pgCentreLon, Boundary: generousZoneBoundary(),
		Sort: SortNewest, Status: "Permitted", Limit: 1,
	}
	_, cursor, err := store.FindInBoundaryZonePage(ctx, permitted)
	if err != nil {
		t.Fatalf("mint status=Permitted cursor: %v", err)
	}
	if cursor == "" {
		t.Fatal("expected a continuation cursor after a full filtered page")
	}

	rejectedReplay := permitted
	rejectedReplay.Status, rejectedReplay.Cursor = "Rejected", cursor
	if _, _, err := store.FindInBoundaryZonePage(ctx, rejectedReplay); !errors.Is(err, ErrCursorFilterMismatch) {
		t.Errorf("replay Permitted cursor under Rejected: got %v, want ErrCursorFilterMismatch", err)
	}

	unfilteredReplay := InBoundaryQuery{
		Latitude: pgCentreLat, Longitude: pgCentreLon, Boundary: generousZoneBoundary(),
		Sort: SortNewest, Cursor: cursor,
	}
	if _, _, err := store.FindInBoundaryZonePage(ctx, unfilteredReplay); !errors.Is(err, ErrCursorFilterMismatch) {
		t.Errorf("replay Permitted cursor unfiltered: got %v, want ErrCursorFilterMismatch", err)
	}
}

// TestPostgresStore_FindInBoundaryZonePage_UnsupportedSort proves an
// out-of-set sort is rejected with ErrUnsupportedSort.
func TestPostgresStore_FindInBoundaryZonePage_UnsupportedSort(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)
	sortFixture(t, store)

	if _, _, err := store.FindInBoundaryZonePage(ctx, boundaryZoneQuery(Sort("nonsense"), 10, "")); !errors.Is(err, ErrUnsupportedSort) {
		t.Errorf("unsupported sort: got err %v, want ErrUnsupportedSort", err)
	}
}

// TestPostgresStore_FindInBoundaryZonePage_CoversPolygonNotEnclosingCircle is
// the honest real-PostGIS proof ADR 0032 requires for the sort/filter-aware
// path, mirroring boundarypage_integration_test.go's proof for the plain
// path: a point inside the polygon matches under every sort, and a point
// inside the polygon's enclosing circle but outside the polygon itself does
// NOT match -- the exact spatial distinction a fake store cannot model.
func TestPostgresStore_FindInBoundaryZonePage_CoversPolygonNotEnclosingCircle(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	boundary := squareBoundary(0.005)

	inside := pgApp("INSIDE", 100)
	inside.Longitude = pgPtr(pgCentreLon + 0.001)
	inside.Latitude = pgPtr(pgCentreLat + 0.001)

	outsideSquareInsideCircle := pgApp("OUTSIDE-SQUARE", 100)
	outsideSquareInsideCircle.Longitude = pgPtr(pgCentreLon + 0.02)
	outsideSquareInsideCircle.Latitude = pgPtr(pgCentreLat + 0.02)

	for _, a := range []PlanningApplication{inside, outsideSquareInsideCircle} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	for _, sort := range []Sort{SortDistance, SortNewest, SortStatus} {
		page, _, err := store.FindInBoundaryZonePage(ctx, InBoundaryQuery{
			Latitude: pgCentreLat, Longitude: pgCentreLon, Boundary: boundary, Sort: sort, Limit: 10,
		})
		if err != nil {
			t.Fatalf("FindInBoundaryZonePage(%s): %v", sort, err)
		}
		if got := appNames(page); !reflect.DeepEqual(got, []string{"INSIDE"}) {
			t.Errorf("%s: got %v, want [INSIDE] (OUTSIDE-SQUARE is inside the enclosing circle but outside the polygon)", sort, got)
		}
	}
}

// TestPostgresStore_FindInBoundaryZonePage_RespectsBoundary proves the
// polygon predicate still bounds every sort: a row outside the boundary never
// appears, even when it would otherwise sort first (e.g. the newest
// start_date).
func TestPostgresStore_FindInBoundaryZonePage_RespectsBoundary(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)
	far := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC) // newest of all, but far away
	for _, a := range []PlanningApplication{
		withStart(at(pgApp("IN", 100), 100), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		withStart(at(pgApp("OUT", 100), 50000), far), // 50 km, well outside the boundary
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}
	for _, sort := range []Sort{SortDistance, SortNewest, SortOldest, SortStatus} {
		got := pageAllInBoundaryZone(t, store, sort, 10)
		if !reflect.DeepEqual(got, []string{"IN"}) {
			t.Errorf("%s: got %v, want [IN] (OUT is beyond the boundary)", sort, got)
		}
	}
}
