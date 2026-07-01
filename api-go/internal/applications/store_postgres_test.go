//go:build integration

package applications

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// Deterministic fixture geometry, mirroring the Phase 0 spatial proof
// (internal/platform/postgres/spatial_integration_test.go): a fixed London
// centre with applications offset due north by a known number of metres, so
// PostGIS's spheroidal distances are exact to within a metre.
const (
	pgCentreLon    = -0.1278
	pgCentreLat    = 51.5074
	pgMetresPerLat = 111_320.0
)

func pgLatNorth(metres float64) float64 { return pgCentreLat + metres/pgMetresPerLat }

// pgPtr returns a pointer to v. Used to populate the nullable pointer fields of
// PlanningApplication in test fixtures.
func pgPtr[T any](v T) *T { return &v }

// newAppPGStore returns a Postgres-backed applications store over a truncated
// database. Integration tests are NOT run in parallel: they share the single
// docker-compose database and TRUNCATE it per test for isolation.
func newAppPGStore(t *testing.T) *PostgresStore {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	return NewPostgresStore(pool)
}

// pgApp builds a baseline application in the given authority with all nullable
// fields left nil and no coordinates. Tests set the fields they exercise.
func pgApp(name string, areaID int) PlanningApplication {
	return PlanningApplication{
		Name:          name,
		UID:           "uid-" + name,
		AreaName:      "Testshire",
		AreaID:        areaID,
		Address:       "1 Test Street",
		Description:   "a planning application",
		LastDifferent: time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
	}
}

// at places the application's centre `metres` due north of the fixture centre.
func at(a PlanningApplication, metres float64) PlanningApplication {
	a.Longitude = pgPtr(pgCentreLon)
	a.Latitude = pgPtr(pgLatNorth(metres))
	return a
}

func assertTimePtrEqual(t *testing.T, field string, got, want *time.Time) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Errorf("%s nil mismatch: got %v, want %v", field, got, want)
		return
	}
	if got != nil && !got.Equal(*want) {
		t.Errorf("%s: got %v, want %v", field, *got, *want)
	}
}

// assertAppEqual compares two snapshots, using time.Equal for the temporal
// fields (a timestamptz/date round-trip can change the time.Time location while
// preserving the instant) and reflect.DeepEqual for everything else.
func assertAppEqual(t *testing.T, got, want PlanningApplication) {
	t.Helper()
	if !got.LastDifferent.Equal(want.LastDifferent) {
		t.Errorf("LastDifferent: got %v, want %v", got.LastDifferent, want.LastDifferent)
	}
	assertTimePtrEqual(t, "StartDate", got.StartDate, want.StartDate)
	assertTimePtrEqual(t, "DecidedDate", got.DecidedDate, want.DecidedDate)
	assertTimePtrEqual(t, "ConsultedDate", got.ConsultedDate, want.ConsultedDate)

	g, w := got, want
	g.LastDifferent, w.LastDifferent = time.Time{}, time.Time{}
	g.StartDate, w.StartDate = nil, nil
	g.DecidedDate, w.DecidedDate = nil, nil
	g.ConsultedDate, w.ConsultedDate = nil, nil
	if !reflect.DeepEqual(g, w) {
		t.Errorf("application mismatch:\n got = %+v\nwant = %+v", g, w)
	}
}

func appNames(apps []PlanningApplication) []string {
	names := make([]string, len(apps))
	for i, a := range apps {
		names[i] = a.Name
	}
	return names
}

func assertNames(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
}

// TestPostgresStore_Upsert_RoundTripFull writes a fully-populated application —
// every nullable field set, coordinates present — and reads it back unchanged.
func TestPostgresStore_Upsert_RoundTripFull(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	start := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	decided := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	consulted := time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC)
	want := PlanningApplication{
		Name:          "24/0001/FUL",
		UID:           "raw-uid-1",
		AreaName:      "Testshire",
		AreaID:        100,
		Address:       "1 Test Street",
		Postcode:      pgPtr("TE1 1ST"),
		Description:   "Single storey rear extension",
		AppType:       pgPtr("Full"),
		AppState:      pgPtr("Permitted"),
		AppSize:       pgPtr("Small"),
		StartDate:     &start,
		DecidedDate:   &decided,
		ConsultedDate: &consulted,
		Longitude:     pgPtr(pgCentreLon),
		Latitude:      pgPtr(pgLatNorth(100)),
		URL:           pgPtr("https://example.test/app/1"),
		Link:          pgPtr("https://example.test/link/1"),
		LastDifferent: time.Date(2026, 6, 26, 9, 30, 0, 0, time.UTC),
	}

	if err := store.Upsert(ctx, want); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, found, err := store.GetByAuthorityAndName(ctx, "100", "24/0001/FUL")
	if err != nil {
		t.Fatalf("GetByAuthorityAndName: %v", err)
	}
	if !found {
		t.Fatal("expected application to be found")
	}
	assertAppEqual(t, got, want)
}

// TestPostgresStore_Upsert_RoundTripNullable writes an application with every
// nullable field absent and no coordinates, and reads back the same nils — the
// NULL-location case PostGIS must preserve.
func TestPostgresStore_Upsert_RoundTripNullable(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	want := pgApp("24/0002/FUL", 100) // all pointers nil, no coords
	if err := store.Upsert(ctx, want); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, found, err := store.GetByAuthorityAndName(ctx, "100", "24/0002/FUL")
	if err != nil {
		t.Fatalf("GetByAuthorityAndName: %v", err)
	}
	if !found {
		t.Fatal("expected application to be found")
	}
	assertAppEqual(t, got, want)
	if got.Latitude != nil || got.Longitude != nil {
		t.Errorf("expected nil coordinates, got lat=%v lon=%v", got.Latitude, got.Longitude)
	}
}

// TestPostgresStore_Upsert_UpdatesInPlace proves ON CONFLICT (authority_code,
// planit_name) updates the existing row rather than inserting a second.
func TestPostgresStore_Upsert_UpdatesInPlace(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	first := pgApp("24/0003/FUL", 100)
	first.AppState = pgPtr("Pending")
	first.Address = "old address"
	if err := store.Upsert(ctx, first); err != nil {
		t.Fatalf("Upsert first: %v", err)
	}

	second := pgApp("24/0003/FUL", 100)
	second.AppState = pgPtr("Permitted")
	second.Address = "new address"
	if err := store.Upsert(ctx, second); err != nil {
		t.Fatalf("Upsert second: %v", err)
	}

	got, found, err := store.GetByAuthorityAndName(ctx, "100", "24/0003/FUL")
	if err != nil || !found {
		t.Fatalf("GetByAuthorityAndName: found=%v err=%v", found, err)
	}
	if got.Address != "new address" || got.AppState == nil || *got.AppState != "Permitted" {
		t.Errorf("expected updated row, got address=%q appState=%v", got.Address, got.AppState)
	}
	// Exactly one row for the authority.
	all, err := store.RecentByAuthority(ctx, "100", 10)
	if err != nil {
		t.Fatalf("RecentByAuthority: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 row after in-place update, got %d", len(all))
	}
}

// TestPostgresStore_Upsert_CompositeKeyAllowsSameNameAcrossAuthorities is the
// schema-correction proof: a PlanIt case reference is only unique within an
// authority, so two authorities sharing the same planit_name must coexist as two
// distinct rows. A global planit_name PRIMARY KEY would silently overwrite one.
func TestPostgresStore_Upsert_CompositeKeyAllowsSameNameAcrossAuthorities(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	a := pgApp("24/SAME/FUL", 100)
	a.UID = "uid-authority-100"
	a.AreaName = "Northshire"
	b := pgApp("24/SAME/FUL", 200)
	b.UID = "uid-authority-200"
	b.AreaName = "Southshire"

	if err := store.Upsert(ctx, a); err != nil {
		t.Fatalf("Upsert a: %v", err)
	}
	if err := store.Upsert(ctx, b); err != nil {
		t.Fatalf("Upsert b: %v", err)
	}

	gotA, foundA, err := store.GetByAuthorityAndName(ctx, "100", "24/SAME/FUL")
	if err != nil || !foundA {
		t.Fatalf("get a: found=%v err=%v", foundA, err)
	}
	gotB, foundB, err := store.GetByAuthorityAndName(ctx, "200", "24/SAME/FUL")
	if err != nil || !foundB {
		t.Fatalf("get b: found=%v err=%v", foundB, err)
	}
	if gotA.UID != "uid-authority-100" || gotA.AreaName != "Northshire" {
		t.Errorf("authority 100 row clobbered: %+v", gotA)
	}
	if gotB.UID != "uid-authority-200" || gotB.AreaName != "Southshire" {
		t.Errorf("authority 200 row clobbered: %+v", gotB)
	}
}

// TestPostgresStore_GetByAuthorityAndName_Miss returns found=false, no error.
func TestPostgresStore_GetByAuthorityAndName_Miss(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	_, found, err := store.GetByAuthorityAndName(ctx, "100", "does-not-exist")
	if err != nil {
		t.Fatalf("expected nil error on miss, got %v", err)
	}
	if found {
		t.Fatal("expected found=false on miss")
	}
}

// TestPostgresStore_GetByUID covers the saved-application backfill lookup: by raw
// uid within an authority, with a miss (and a wrong-authority miss) returning
// found=false and no error.
func TestPostgresStore_GetByUID(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	a := pgApp("24/0004/FUL", 100)
	a.UID = "the-uid"
	if err := store.Upsert(ctx, a); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, found, err := store.GetByUID(ctx, "the-uid", "100")
	if err != nil || !found {
		t.Fatalf("GetByUID hit: found=%v err=%v", found, err)
	}
	if got.Name != "24/0004/FUL" {
		t.Errorf("GetByUID returned wrong app: %+v", got)
	}

	if _, found, err := store.GetByUID(ctx, "no-such-uid", "100"); err != nil || found {
		t.Errorf("GetByUID miss: found=%v err=%v", found, err)
	}
	if _, found, err := store.GetByUID(ctx, "the-uid", "200"); err != nil || found {
		t.Errorf("GetByUID wrong-authority: found=%v err=%v", found, err)
	}
}

// TestPostgresStore_RecentByAuthority orders by last_different DESC, bounds by
// cap, and scopes to the authority.
func TestPostgresStore_RecentByAuthority(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	older := pgApp("OLD", 100)
	older.LastDifferent = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	mid := pgApp("MID", 100)
	mid.LastDifferent = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	newer := pgApp("NEW", 100)
	newer.LastDifferent = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	other := pgApp("OTHER-AUTH", 200) // must not appear for authority 100
	for _, a := range []PlanningApplication{older, mid, newer, other} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	got, err := store.RecentByAuthority(ctx, "100", 10)
	if err != nil {
		t.Fatalf("RecentByAuthority: %v", err)
	}
	assertNames(t, appNames(got), []string{"NEW", "MID", "OLD"})

	capped, err := store.RecentByAuthority(ctx, "100", 2)
	if err != nil {
		t.Fatalf("RecentByAuthority capped: %v", err)
	}
	assertNames(t, appNames(capped), []string{"NEW", "MID"})
}

// TestPostgresStore_BreakdownByAuthority returns exact per-app_state counts
// including the NULL-app_state bucket, sorted by sortStateCounts, scoped to the
// authority and summing to the true total.
func TestPostgresStore_BreakdownByAuthority(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	apps := []PlanningApplication{
		withState(pgApp("P1", 100), pgPtr("Permitted")),
		withState(pgApp("P2", 100), pgPtr("Permitted")),
		withState(pgApp("R1", 100), pgPtr("Rejected")),
		withState(pgApp("N1", 100), nil), // NULL app_state
		withState(pgApp("OTHER", 200), pgPtr("Permitted")),
	}
	for _, a := range apps {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	got, err := store.BreakdownByAuthority(ctx, "100")
	if err != nil {
		t.Fatalf("BreakdownByAuthority: %v", err)
	}
	want := []StateCount{
		{AppState: pgPtr("Permitted"), Count: 2},
		{AppState: pgPtr("Rejected"), Count: 1},
		{AppState: nil, Count: 1},
	}
	assertBreakdown(t, got, want)
}

func withState(a PlanningApplication, state *string) PlanningApplication {
	a.AppState = state
	return a
}

func assertBreakdown(t *testing.T, got, want []StateCount) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("breakdown length = %d, want %d (%+v)", len(got), len(want), got)
	}
	total := 0
	for i := range want {
		if (got[i].AppState == nil) != (want[i].AppState == nil) {
			t.Fatalf("bucket %d appState nil mismatch: got %v want %v", i, got[i].AppState, want[i].AppState)
		}
		if got[i].AppState != nil && *got[i].AppState != *want[i].AppState {
			t.Fatalf("bucket %d appState: got %q want %q", i, *got[i].AppState, *want[i].AppState)
		}
		if got[i].Count != want[i].Count {
			t.Fatalf("bucket %d count: got %d want %d", i, got[i].Count, want[i].Count)
		}
		total += got[i].Count
	}
	wantTotal := 0
	for _, w := range want {
		wantTotal += w.Count
	}
	if total != wantTotal {
		t.Fatalf("breakdown total = %d, want %d", total, wantTotal)
	}
}

// TestPostgresStore_FindNearbyPage proves nearest-first ordering, the radius
// filter, and keyset pagination across pages with no overlap or gap and an empty
// next-cursor at exhaustion.
func TestPostgresStore_FindNearbyPage(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	for _, a := range []PlanningApplication{
		at(pgApp("APP-100", 100), 100),
		at(pgApp("APP-500", 100), 500),
		at(pgApp("APP-5000", 100), 5000),
		at(pgApp("APP-FAR", 100), 50000), // outside any test radius
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	// Radius filter + nearest-first ordering, first page.
	page1, cursor1, err := store.FindNearbyPage(ctx, pgCentreLat, pgCentreLon, 6000, 2, "")
	if err != nil {
		t.Fatalf("FindNearbyPage page1: %v", err)
	}
	assertNames(t, appNames(page1), []string{"APP-100", "APP-500"})
	if cursor1 == "" {
		t.Fatal("expected a continuation cursor after a full page")
	}

	page2, cursor2, err := store.FindNearbyPage(ctx, pgCentreLat, pgCentreLon, 6000, 2, cursor1)
	if err != nil {
		t.Fatalf("FindNearbyPage page2: %v", err)
	}
	assertNames(t, appNames(page2), []string{"APP-5000"})
	if cursor2 != "" {
		t.Fatalf("expected empty cursor at exhaustion, got %q", cursor2)
	}
	// APP-FAR (50 km) is excluded by the 6 km radius.
}

// TestPostgresStore_FindNearbyPage_TieBreak proves the planit_name tie-break
// keeps keyset pagination correct when two applications share an exact distance.
func TestPostgresStore_FindNearbyPage_TieBreak(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	for _, a := range []PlanningApplication{
		at(pgApp("APP-1-100", 100), 100),
		at(pgApp("APP-2-200A", 100), 200),
		at(pgApp("APP-3-200B", 100), 200), // identical distance to 200A
		at(pgApp("APP-4-500", 100), 500),
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	var collected []string
	cursor := ""
	for range 10 { // safety bound
		page, next, err := store.FindNearbyPage(ctx, pgCentreLat, pgCentreLon, 1000, 1, cursor)
		if err != nil {
			t.Fatalf("FindNearbyPage: %v", err)
		}
		collected = append(collected, appNames(page)...)
		if next == "" {
			break
		}
		cursor = next
	}
	assertNames(t, collected, []string{"APP-1-100", "APP-2-200A", "APP-3-200B", "APP-4-500"})
}

// withStart sets the application's start_date.
func withStart(a PlanningApplication, d time.Time) PlanningApplication {
	a.StartDate = &d
	return a
}

// zoneQuery is a brief InZoneQuery builder fixed to the standard 6 km London
// fixture centre, for the cursor/sort-error tests.
func zoneQuery(sort Sort, limit int, cursor string) InZoneQuery {
	return InZoneQuery{
		Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 6000,
		Sort: sort, Limit: limit, Cursor: cursor,
	}
}

// pageAllFiltered pages FindInZonePage to exhaustion for an arbitrary query
// (sort + status/unread filter + page size), returning the names in the order
// seen across pages. base.Cursor is ignored (paging starts from page one). It
// bounds the loop so a keyset bug that fails to advance fails instead of hanging.
func pageAllFiltered(t *testing.T, store *PostgresStore, base InZoneQuery) []string {
	t.Helper()
	ctx := context.Background()
	var names []string
	cursor := ""
	for range 1000 {
		q := base
		q.Cursor = cursor
		page, next, err := store.FindInZonePage(ctx, q)
		if err != nil {
			t.Fatalf("FindInZonePage(sort=%s status=%q unread=%t cursor=%q): %v", base.Sort, base.Status, base.Unread, cursor, err)
		}
		names = append(names, appNames(page)...)
		if next == "" {
			return names
		}
		cursor = next
	}
	t.Fatalf("FindInZonePage(sort=%s status=%q unread=%t) did not exhaust within the safety bound", base.Sort, base.Status, base.Unread)
	return nil
}

// pageAllInZone pages FindInZonePage to exhaustion as an anonymous caller (no
// userID), for the sorts that ignore per-user data.
func pageAllInZone(t *testing.T, store *PostgresStore, sort Sort, radius float64, limit int) []string {
	t.Helper()
	return pageAllInZoneAs(t, store, "", sort, radius, limit)
}

// pageAllInZoneAs pages FindInZonePage to exhaustion for the given caller and
// page size and returns the names in the order seen across pages. It bounds the
// loop so a keyset bug that fails to advance fails the test instead of hanging.
func pageAllInZoneAs(t *testing.T, store *PostgresStore, userID string, sort Sort, radius float64, limit int) []string {
	t.Helper()
	ctx := context.Background()
	var names []string
	cursor := ""
	for range 1000 {
		page, next, err := store.FindInZonePage(ctx, InZoneQuery{
			UserID: userID, Latitude: pgCentreLat, Longitude: pgCentreLon,
			RadiusMetres: radius, Sort: sort, Limit: limit, Cursor: cursor,
		})
		if err != nil {
			t.Fatalf("FindInZonePage(%s, cursor=%q): %v", sort, cursor, err)
		}
		names = append(names, appNames(page)...)
		if next == "" {
			return names
		}
		cursor = next
	}
	t.Fatalf("FindInZonePage(%s) did not exhaust within the safety bound", sort)
	return nil
}

// sortFixture seeds a deterministic in-radius set spanning three start_dates
// (one shared across two authorities, to exercise the (authority_code,
// planit_name) tiebreak) plus two NULL-start_date rows (to exercise NULLS LAST
// and a page boundary inside the NULL tail). All rows are within 6 km.
func sortFixture(t *testing.T, store *PostgresStore) {
	t.Helper()
	ctx := context.Background()
	mar := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, a := range []PlanningApplication{
		withStart(at(pgApp("A1", 100), 100), mar),
		withStart(at(pgApp("A2", 100), 200), jan),
		withStart(at(pgApp("A3", 100), 300), feb),
		withStart(at(pgApp("A4", 200), 350), feb), // equal date to A3, other authority
		at(pgApp("A5", 100), 400),                 // NULL start_date
		at(pgApp("A6", 100), 500),                 // NULL start_date
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}
}

// TestPostgresStore_FindInZonePage_Distance proves the sort-aware path reproduces
// the legacy nearest-first ordering for SortDistance and pages without gaps.
func TestPostgresStore_FindInZonePage_Distance(t *testing.T) {
	store := newAppPGStore(t)
	sortFixture(t, store)

	// Single page (canonical order) — nearest-first by construction distance.
	want := []string{"A1", "A2", "A3", "A4", "A5", "A6"}
	if got := pageAllInZone(t, store, SortDistance, 6000, 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("distance single page: got %v, want %v", got, want)
	}
	// Paged to exhaustion with a small page size — must equal the canonical order.
	if got := pageAllInZone(t, store, SortDistance, 6000, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("distance paged: got %v, want %v", got, want)
	}
}

// TestPostgresStore_FindInZonePage_Newest proves start_date DESC NULLS LAST with
// the (authority_code, planit_name) tiebreak, paged to exhaustion with no overlap
// or gap, matches the single unpaged order.
func TestPostgresStore_FindInZonePage_Newest(t *testing.T) {
	store := newAppPGStore(t)
	sortFixture(t, store)

	// DESC by start_date; equal date (A3/A4) tiebreaks on authority_code "100" <
	// "200"; NULL start_date (A5/A6) sorts last on planit_name "A5" < "A6".
	want := []string{"A1", "A3", "A4", "A2", "A5", "A6"}
	if got := pageAllInZone(t, store, SortNewest, 6000, 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("newest single page: got %v, want %v", got, want)
	}
	if got := pageAllInZone(t, store, SortNewest, 6000, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("newest paged: got %v, want %v", got, want)
	}
	// Page size 1 forces a keyset boundary at every row, including inside the NULL tail.
	if got := pageAllInZone(t, store, SortNewest, 6000, 1); !reflect.DeepEqual(got, want) {
		t.Fatalf("newest paged size 1: got %v, want %v", got, want)
	}
}

// TestPostgresStore_FindInZonePage_Oldest proves start_date ASC NULLS LAST with
// the same tiebreak, paged to exhaustion, matches the single unpaged order.
func TestPostgresStore_FindInZonePage_Oldest(t *testing.T) {
	store := newAppPGStore(t)
	sortFixture(t, store)

	want := []string{"A2", "A3", "A4", "A1", "A5", "A6"}
	if got := pageAllInZone(t, store, SortOldest, 6000, 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("oldest single page: got %v, want %v", got, want)
	}
	if got := pageAllInZone(t, store, SortOldest, 6000, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("oldest paged: got %v, want %v", got, want)
	}
	if got := pageAllInZone(t, store, SortOldest, 6000, 1); !reflect.DeepEqual(got, want) {
		t.Fatalf("oldest paged size 1: got %v, want %v", got, want)
	}
}

// statusFixture seeds a deterministic in-radius set spanning two non-NULL
// app_state values ("Permitted" < "Rejected") plus a NULL-app_state group, each
// with varied and NULL start_dates, including an equal-(app_state, start_date)
// pair across two authorities to exercise the (authority_code, planit_name)
// tiebreak. All rows are within 6 km.
func statusFixture(t *testing.T, store *PostgresStore) {
	t.Helper()
	ctx := context.Background()
	mar := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	permitted, rejected := pgPtr("Permitted"), pgPtr("Rejected")
	for _, a := range []PlanningApplication{
		withState(withStart(at(pgApp("P1", 100), 100), mar), permitted),
		withState(withStart(at(pgApp("P2", 100), 200), jan), permitted),
		withState(withStart(at(pgApp("P3", 100), 300), feb), permitted),
		withState(withStart(at(pgApp("P4", 200), 350), feb), permitted), // equal (state,date) as P3, other authority
		withState(at(pgApp("P5", 100), 400), permitted),                 // NULL start_date within Permitted
		withState(withStart(at(pgApp("R1", 100), 500), feb), rejected),
		withState(at(pgApp("R2", 100), 600), rejected), // NULL start_date within Rejected
		withStart(at(pgApp("N1", 100), 700), mar),      // NULL app_state
		at(pgApp("N2", 100), 800),                      // NULL app_state, NULL start_date
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}
}

// TestPostgresStore_FindInZonePage_Status proves the mixed-direction keyset —
// app_state ASC NULLS LAST, start_date DESC NULLS LAST, (authority_code,
// planit_name) — pages to exhaustion with no overlap or gap and matches the single
// unpaged order. NULL app_state rows sort last; within an app_state group NULL
// start_date rows sort last; equal (app_state, start_date) tiebreaks on
// (authority_code, planit_name).
func TestPostgresStore_FindInZonePage_Status(t *testing.T) {
	store := newAppPGStore(t)
	statusFixture(t, store)

	// Permitted (state ASC first): mar, feb tiebreak "100"(P3)<"200"(P4), jan, NULL.
	// Rejected next: feb, NULL. NULL app_state last: mar, NULL.
	want := []string{"P1", "P3", "P4", "P2", "P5", "R1", "R2", "N1", "N2"}
	if got := pageAllInZone(t, store, SortStatus, 6000, 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("status single page: got %v, want %v", got, want)
	}
	if got := pageAllInZone(t, store, SortStatus, 6000, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("status paged: got %v, want %v", got, want)
	}
	// Page size 1 forces a keyset boundary at every row: across app_state group
	// boundaries, inside a group's NULL-start_date tail, and into the NULL
	// app_state tail.
	if got := pageAllInZone(t, store, SortStatus, 6000, 1); !reflect.DeepEqual(got, want) {
		t.Fatalf("status paged size 1: got %v, want %v", got, want)
	}
	// Page size 3 lands boundaries mid-group and at group edges.
	if got := pageAllInZone(t, store, SortStatus, 6000, 3); !reflect.DeepEqual(got, want) {
		t.Fatalf("status paged size 3: got %v, want %v", got, want)
	}
}

// TestPostgresStore_FindInZonePage_RespectsRadius proves the spatial predicate
// still bounds every sort: a row outside the radius never appears.
func TestPostgresStore_FindInZonePage_RespectsRadius(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)
	far := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC) // newest of all, but far away
	for _, a := range []PlanningApplication{
		withStart(at(pgApp("IN", 100), 100), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		withStart(at(pgApp("OUT", 100), 50000), far), // 50 km, outside 6 km
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}
	for _, sort := range []Sort{SortDistance, SortNewest, SortOldest, SortStatus} {
		got := pageAllInZone(t, store, sort, 6000, 10)
		if !reflect.DeepEqual(got, []string{"IN"}) {
			t.Errorf("%s: got %v, want [IN] (OUT is beyond the radius)", sort, got)
		}
	}
}

// TestPostgresStore_FindInZonePage_CursorSortMismatch proves a cursor minted under
// one sort is rejected when replayed under another, and a malformed cursor is
// reported as ErrCursorInvalid — both so the handler returns 400, never a
// mis-ordered page.
func TestPostgresStore_FindInZonePage_CursorSortMismatch(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)
	sortFixture(t, store)

	_, cursor, err := store.FindInZonePage(ctx, zoneQuery(SortNewest, 1, ""))
	if err != nil {
		t.Fatalf("mint newest cursor: %v", err)
	}
	if cursor == "" {
		t.Fatal("expected a continuation cursor after a full page")
	}

	if _, _, err := store.FindInZonePage(ctx, zoneQuery(SortOldest, 1, cursor)); !errors.Is(err, ErrCursorSortMismatch) {
		t.Errorf("replay newest cursor under oldest: got err %v, want ErrCursorSortMismatch", err)
	}

	// A cursor minted under status carries mode=status; replaying it under any
	// other sort is rejected, never a mis-ordered page.
	_, statusCursor, err := store.FindInZonePage(ctx, zoneQuery(SortStatus, 1, ""))
	if err != nil {
		t.Fatalf("mint status cursor: %v", err)
	}
	if statusCursor == "" {
		t.Fatal("expected a continuation cursor after a full status page")
	}
	if _, _, err := store.FindInZonePage(ctx, zoneQuery(SortNewest, 1, statusCursor)); !errors.Is(err, ErrCursorSortMismatch) {
		t.Errorf("replay status cursor under newest: got err %v, want ErrCursorSortMismatch", err)
	}

	if _, _, err := store.FindInZonePage(ctx, zoneQuery(SortNewest, 1, "!!!not-base64!!!")); !errors.Is(err, ErrCursorInvalid) {
		t.Errorf("malformed cursor: got err %v, want ErrCursorInvalid", err)
	}
}

// TestPostgresStore_FindInZonePage_UnsupportedSort proves an out-of-set sort is
// rejected with ErrUnsupportedSort (the handler validates first, but the store
// fails closed rather than silently running an arbitrary order).
func TestPostgresStore_FindInZonePage_UnsupportedSort(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)
	sortFixture(t, store)

	if _, _, err := store.FindInZonePage(ctx, zoneQuery(Sort("nonsense"), 10, "")); !errors.Is(err, ErrUnsupportedSort) {
		t.Errorf("unsupported sort: got err %v, want ErrUnsupportedSort", err)
	}
}

// newActivityPGStore returns a store plus its pool over a database truncated of
// applications AND the per-user notification tables, so the recent-activity join
// starts from a clean state. The pool is returned for raw notification seeding
// (the store has no notification writer).
func newActivityPGStore(t *testing.T) (*PostgresStore, *pgxpool.Pool) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "notifications", "notification_state", "watch_zones")
	return NewPostgresStore(pool), pool
}

// seedWatermark inserts the caller's notification_state row. Under read_at (ADR
// 0035) the recent-activity/unread queries no longer JOIN this table; the row is
// seeded so backfillReadAt can derive read_at from it, exactly reproducing the
// old watermark semantics for the equivalence assertions.
func seedWatermark(t *testing.T, pool *pgxpool.Pool, userID string, lastReadAt time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		"INSERT INTO notification_state (user_id, last_read_at) VALUES ($1, $2)",
		userID, lastReadAt); err != nil {
		t.Fatalf("seed watermark for %q: %v", userID, err)
	}
}

// seedNotification inserts one UNREAD notification for the caller (read_at NULL —
// the pre-backfill state). authorityID is the INTEGER authority_id the
// recent-activity subquery groups on and the outer join matches against the
// application's area_id (NOT the text authority_code).
func seedNotification(t *testing.T, pool *pgxpool.Pool, id, userID, appUID string, authorityID int, createdAt time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		"INSERT INTO notifications (id, user_id, application_uid, authority_id, event_type, created_at) "+
			"VALUES ($1, $2, $3, $4, $5, $6)",
		id, userID, appUID, authorityID, "NewApplication", createdAt); err != nil {
		t.Fatalf("seed notification %q: %v", id, err)
	}
}

// backfillReadAt replays migration 0015's two backfill UPDATEs, deriving read_at
// from the seeded watermark rows. Calling it after a watermark+notification
// fixture makes read_at IS NULL reproduce the old created_at > last_read_at unread
// set exactly — the equivalence acceptance criterion (ADR 0035). A notification for
// a user with NO watermark row is marked read (read_at = created_at), matching the
// old "no notification_state ⇒ no unread" behaviour for existing users.
func backfillReadAt(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
UPDATE notifications n
SET read_at = ns.last_read_at
FROM notification_state ns
WHERE n.user_id = ns.user_id
  AND n.created_at <= ns.last_read_at
  AND n.read_at IS NULL`); err != nil {
		t.Fatalf("backfill watermarked: %v", err)
	}
	if _, err := pool.Exec(ctx, `
UPDATE notifications n
SET read_at = n.created_at
WHERE n.read_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM notification_state ns WHERE ns.user_id = n.user_id)`); err != nil {
		t.Fatalf("backfill no-watermark: %v", err)
	}
}

// activityFixture seeds a deterministic in-radius set for the recent-activity
// sort. The headline case: AA has the oldest start_date but a fresh UNREAD event,
// so its activity timestamp (the unread's created_at) floats it above BB, which
// has the newest start_date but no unread. CC and DD share an activity timestamp
// (equal start_date, no unread) across two authorities to exercise the
// (authority_code, planit_name) tiebreak; EE and FF have neither start_date nor
// unread, so their activity is NULL (the NULLS-LAST tail).
func activityFixture(t *testing.T, store *PostgresStore, pool *pgxpool.Pool, userID string) {
	t.Helper()
	ctx := context.Background()
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	jun := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for _, a := range []PlanningApplication{
		withStart(at(pgApp("AA", 100), 100), jan), // oldest start, but a fresh unread
		withStart(at(pgApp("BB", 100), 200), jun), // newest start, no unread
		withStart(at(pgApp("CC", 100), 300), feb), // equal activity to DD
		withStart(at(pgApp("DD", 200), 350), feb), // equal date, other authority
		at(pgApp("EE", 100), 400),                 // NULL start, no unread -> NULL activity
		at(pgApp("FF", 100), 500),                 // NULL start, no unread -> NULL activity
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}
	// Watermark well before the unread event so AA's notification is unread.
	seedWatermark(t, pool, userID, jan)
	// AA's unread event, later than BB's newest start_date, floats AA to the top.
	seedNotification(t, pool, "n-AA", userID, "uid-AA", 100, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))
	// Derive read_at from the watermark: n-AA (after jan) stays unread (read_at NULL).
	backfillReadAt(t, pool)
}

// TestPostgresStore_FindInZonePage_RecentActivity proves the GREATEST(start_date,
// unread.created_at) DESC NULLS LAST ordering with the (authority_code,
// planit_name) tiebreak, paged to exhaustion with no overlap or gap, matches the
// single unpaged order. The headline behaviour: an app with a fresh UNREAD event
// (AA) sorts above an app with a newer start_date but no unread (BB).
func TestPostgresStore_FindInZonePage_RecentActivity(t *testing.T) {
	store, pool := newActivityPGStore(t)
	const userID = "user-A"
	activityFixture(t, store, pool, userID)

	// AA(unread jun15) > BB(start jun01) > CC(feb,auth100) > DD(feb,auth200) >
	// EE(NULL) > FF(NULL). AA above BB is the headline unread-floats-up case;
	// CC before DD is the equal-activity authority tiebreak; EE/FF are the
	// NULL-activity tail ordered by planit_name.
	want := []string{"AA", "BB", "CC", "DD", "EE", "FF"}
	if got := pageAllInZoneAs(t, store, userID, SortRecentActivity, 6000, 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("recent-activity single page: got %v, want %v", got, want)
	}
	if got := pageAllInZoneAs(t, store, userID, SortRecentActivity, 6000, 2); !reflect.DeepEqual(got, want) {
		t.Fatalf("recent-activity paged: got %v, want %v", got, want)
	}
	// Page size 1 forces a keyset boundary at every row, including inside the
	// NULL-activity tail.
	if got := pageAllInZoneAs(t, store, userID, SortRecentActivity, 6000, 1); !reflect.DeepEqual(got, want) {
		t.Fatalf("recent-activity paged size 1: got %v, want %v", got, want)
	}
}

// TestPostgresStore_FindInZonePage_RecentActivity_JoinKey proves the join keys on
// (application_uid AND authority_id->area_id): a notification whose application_uid
// matches but whose authority_id differs from the application's area_id must NOT
// contribute to that app's activity. If it falsely joined, X's fresh unread would
// float it above Y; correctly it does not, so Y (newer start_date) sorts first.
func TestPostgresStore_FindInZonePage_RecentActivity_JoinKey(t *testing.T) {
	store, pool := newActivityPGStore(t)
	ctx := context.Background()
	const userID = "user-A"
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	for _, a := range []PlanningApplication{
		withStart(at(pgApp("X", 100), 100), jan), // area_id 100
		withStart(at(pgApp("Y", 100), 200), feb), // newer start, no notification
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}
	seedWatermark(t, pool, userID, jan)
	// A fresh unread for X's uid but under the WRONG authority (999 != 100). The
	// join requires area_id = authority_id, so this must not touch X's activity.
	seedNotification(t, pool, "n-wrong", userID, "uid-X", 999, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))
	backfillReadAt(t, pool) // n-wrong (after jan) stays unread, but under authority 999.

	want := []string{"Y", "X"} // Y(feb) > X(jan); the wrong-authority unread is ignored.
	if got := pageAllInZoneAs(t, store, userID, SortRecentActivity, 6000, 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("recent-activity join-key: got %v, want %v (a wrong-authority notification must not join)", got, want)
	}
}

// TestPostgresStore_FindInZonePage_RecentActivity_ExistingNoWatermark proves an
// existing (pre-migration) caller with NO notification_state row has no unread: the
// 0015 backfill marked their history read (read_at = created_at), so read_at IS NULL
// yields nothing and recent-activity orders by start_date alone. This is the
// equivalence to the old "no notification_state ⇒ no unread" behaviour (ADR 0035).
func TestPostgresStore_FindInZonePage_RecentActivity_ExistingNoWatermark(t *testing.T) {
	store, pool := newActivityPGStore(t)
	ctx := context.Background()
	const userID = "user-firsttouch"
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	for _, a := range []PlanningApplication{
		withStart(at(pgApp("P", 100), 100), jan), // matching notification, but no watermark
		withStart(at(pgApp("Q", 100), 200), feb), // newer start, no notification
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}
	// NOTE: no seedWatermark — the user has never read, so has no state row.
	seedNotification(t, pool, "n-P", userID, "uid-P", 100, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))
	backfillReadAt(t, pool) // no-watermark backfill marks n-P read (read_at = created_at).

	want := []string{"Q", "P"} // Q(feb) > P(jan); the backfilled notification is read.
	if got := pageAllInZoneAs(t, store, userID, SortRecentActivity, 6000, 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("recent-activity existing-no-watermark: got %v, want %v (backfilled history is read)", got, want)
	}
}

// TestPostgresStore_FindInZonePage_RecentActivity_NewUnreadSurfaces proves the new
// semantic: a genuinely-new caller (no watermark, no backfill) whose notification is
// unread (read_at IS NULL) DOES surface it — the unread event floats the app above a
// newer-start_date app with no unread. This is the behaviour change GET-no-seed
// enables (ADR 0035).
func TestPostgresStore_FindInZonePage_RecentActivity_NewUnreadSurfaces(t *testing.T) {
	store, pool := newActivityPGStore(t)
	ctx := context.Background()
	const userID = "user-new"
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	for _, a := range []PlanningApplication{
		withStart(at(pgApp("P", 100), 100), jan), // oldest start, fresh unread
		withStart(at(pgApp("Q", 100), 200), feb), // newer start, no notification
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}
	// No watermark, no backfill: the notification stays unread (read_at IS NULL).
	seedNotification(t, pool, "n-P", userID, "uid-P", 100, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))

	want := []string{"P", "Q"} // P's unread (jun15) floats it above Q(feb).
	if got := pageAllInZoneAs(t, store, userID, SortRecentActivity, 6000, 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("recent-activity new-unread: got %v, want %v (a new unread must surface)", got, want)
	}
}

// TestPostgresStore_FindInZonePage_RecentActivity_AtWatermarkIsRead proves a
// notification created exactly at the watermark is read after backfill (the 0015
// UPDATE uses created_at <= last_read_at), so read_at IS NULL excludes it — the
// equivalence of the old strict created_at > last_read_at predicate (ADR 0035).
func TestPostgresStore_FindInZonePage_RecentActivity_AtWatermarkIsRead(t *testing.T) {
	store, pool := newActivityPGStore(t)
	ctx := context.Background()
	const userID = "user-strict"
	watermark := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	for _, a := range []PlanningApplication{
		withStart(at(pgApp("M", 100), 100), jan), // notification exactly AT the watermark
		withStart(at(pgApp("N", 100), 200), feb), // newer start, no notification
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}
	seedWatermark(t, pool, userID, watermark)
	// created_at == last_read_at: backfilled read (created_at <= last_read_at), so
	// M's activity is its start_date, not this timestamp.
	seedNotification(t, pool, "n-M", userID, "uid-M", 100, watermark)
	backfillReadAt(t, pool)

	want := []string{"N", "M"} // N(feb) > M(jan); the at-watermark notification is read.
	if got := pageAllInZoneAs(t, store, userID, SortRecentActivity, 6000, 100); !reflect.DeepEqual(got, want) {
		t.Fatalf("recent-activity at-watermark: got %v, want %v (a notification exactly at the watermark is read)", got, want)
	}
}

// TestPostgresStore_FindInZonePage_StatusFilter proves ?status= restricts the set
// to exactly the matching app_state, composes with every scalar sort, and pages to
// exhaustion with no overlap or gap. Rejected and NULL-app_state rows never appear.
func TestPostgresStore_FindInZonePage_StatusFilter(t *testing.T) {
	store := newAppPGStore(t)
	statusFixture(t, store)

	// Permitted rows only. Under newest/status: start_date DESC NULLS LAST then
	// (authority_code, planit_name) — P1(mar), P3(feb,100), P4(feb,200), P2(jan),
	// P5(NULL). Under distance: nearest-first by construction — P1..P5.
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
			base := InZoneQuery{
				Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 6000,
				Sort: tc.sort, Status: "Permitted", Limit: limit,
			}
			if got := pageAllFiltered(t, store, base); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("status=Permitted sort=%s limit=%d: got %v, want %v", tc.sort, limit, got, tc.want)
			}
		}
	}

	// Rejected rows only, newest: R1(feb), R2(NULL).
	rejected := pageAllFiltered(t, store, InZoneQuery{
		Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 6000,
		Sort: SortNewest, Status: "Rejected", Limit: 1,
	})
	if want := []string{"R1", "R2"}; !reflect.DeepEqual(rejected, want) {
		t.Fatalf("status=Rejected: got %v, want %v", rejected, want)
	}

	// A status with no matching rows yields the empty set.
	empty := pageAllFiltered(t, store, InZoneQuery{
		Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 6000,
		Sort: SortNewest, Status: "Withdrawn", Limit: 10,
	})
	if len(empty) != 0 {
		t.Fatalf("status=Withdrawn (no rows): got %v, want empty", empty)
	}
}

// unreadFixture seeds a deterministic in-radius set for the unread filter under a
// scalar sort. U1 and U4 have a fresh UNREAD notification; U2 has only a READ one
// (at/under the watermark); U3 has none. So ?unread=true keeps only {U1, U4}.
// Distances increase with the name so the distance order is U1..U4.
func unreadFixture(t *testing.T, store *PostgresStore, pool *pgxpool.Pool, userID string) {
	t.Helper()
	ctx := context.Background()
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	mar := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	apr := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	for _, a := range []PlanningApplication{
		withStart(at(pgApp("U1", 100), 100), jan), // fresh unread
		withStart(at(pgApp("U2", 100), 200), feb), // read-only notification
		withStart(at(pgApp("U3", 100), 300), mar), // no notification
		withStart(at(pgApp("U4", 100), 400), apr), // fresh unread
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}
	watermark := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	seedWatermark(t, pool, userID, watermark)
	seedNotification(t, pool, "n-U1", userID, "uid-U1", 100, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)) // > watermark: unread
	seedNotification(t, pool, "n-U2", userID, "uid-U2", 100, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))  // < watermark: read
	seedNotification(t, pool, "n-U4", userID, "uid-U4", 100, time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)) // > watermark: unread
	// Backfill derives read_at: n-U2 (<= watermark) becomes read; n-U1/n-U4 stay unread.
	backfillReadAt(t, pool)
}

// TestPostgresStore_FindInZonePage_UnreadFilter proves ?unread=true (the INNER
// JOIN to the caller's unread notifications) keeps only applications with an
// unread notification, composes with scalar sorts, and pages without gaps. A
// read-only or notification-less application is dropped.
func TestPostgresStore_FindInZonePage_UnreadFilter(t *testing.T) {
	store, pool := newActivityPGStore(t)
	const userID = "user-unread"
	unreadFixture(t, store, pool, userID)

	// Newest: U4(apr) then U1(jan). Distance: U1(100) then U4(400). U2/U3 excluded.
	cases := []struct {
		sort Sort
		want []string
	}{
		{SortNewest, []string{"U4", "U1"}},
		{SortDistance, []string{"U1", "U4"}},
	}
	for _, tc := range cases {
		for _, limit := range []int{100, 1} {
			base := InZoneQuery{
				UserID: userID, Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 6000,
				Sort: tc.sort, Unread: true, Limit: limit,
			}
			if got := pageAllFiltered(t, store, base); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("unread=true sort=%s limit=%d: got %v, want %v", tc.sort, limit, got, tc.want)
			}
		}
	}
}

// TestPostgresStore_FindInZonePage_UnreadFilter_ExistingNoWatermark proves an
// existing (pre-migration) caller with NO notification_state row gets the empty set
// under ?unread=true: the 0015 backfill marked their history read, so read_at IS NULL
// yields nothing (ADR 0035 equivalence to the old INNER-JOIN behaviour).
func TestPostgresStore_FindInZonePage_UnreadFilter_ExistingNoWatermark(t *testing.T) {
	store, pool := newActivityPGStore(t)
	ctx := context.Background()
	const userID = "user-firsttouch"
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	app := withStart(at(pgApp("Z1", 100), 100), jan)
	if err := store.Upsert(ctx, app); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	seedNotification(t, pool, "n-Z1", userID, "uid-Z1", 100, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))
	backfillReadAt(t, pool) // no-watermark backfill marks n-Z1 read.

	got := pageAllFiltered(t, store, InZoneQuery{
		UserID: userID, Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 6000,
		Sort: SortNewest, Unread: true, Limit: 10,
	})
	if len(got) != 0 {
		t.Fatalf("existing-no-watermark unread=true: got %v, want empty (backfilled history is read)", got)
	}
}

// TestPostgresStore_FindInZonePage_UnreadFilter_NewUnreadSurfaces proves the new
// semantic through the dynamic filtered-query builder: a genuinely-new caller (no
// watermark, no backfill) whose notification is unread (read_at IS NULL) keeps the
// app under ?unread=true.
func TestPostgresStore_FindInZonePage_UnreadFilter_NewUnreadSurfaces(t *testing.T) {
	store, pool := newActivityPGStore(t)
	ctx := context.Background()
	const userID = "user-new"
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	app := withStart(at(pgApp("Z1", 100), 100), jan)
	if err := store.Upsert(ctx, app); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	// No watermark, no backfill: the notification stays unread (read_at IS NULL).
	seedNotification(t, pool, "n-Z1", userID, "uid-Z1", 100, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))

	got := pageAllFiltered(t, store, InZoneQuery{
		UserID: userID, Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 6000,
		Sort: SortNewest, Unread: true, Limit: 10,
	})
	if want := []string{"Z1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("new-unread unread=true: got %v, want %v (a new unread must be kept)", got, want)
	}
}

// TestPostgresStore_FindInZonePage_UnreadFilter_RecentActivity proves the unread
// filter composes with the recent-activity sort: the unread join becomes INNER (so
// an application with no unread is dropped even when its start_date is newest),
// while the GREATEST(start_date, unread.created_at) ordering still holds.
func TestPostgresStore_FindInZonePage_UnreadFilter_RecentActivity(t *testing.T) {
	store, pool := newActivityPGStore(t)
	ctx := context.Background()
	const userID = "user-ra-unread"
	jan := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	feb := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	jun := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for _, a := range []PlanningApplication{
		withStart(at(pgApp("AA", 100), 100), jan), // oldest start, fresh unread → top
		withStart(at(pgApp("BB", 100), 200), jun), // newest start, NO unread → dropped by INNER
		withStart(at(pgApp("CC", 100), 300), feb), // unread (older) → second
	} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}
	seedWatermark(t, pool, userID, jan)
	seedNotification(t, pool, "n-AA", userID, "uid-AA", 100, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)) // activity jun15
	seedNotification(t, pool, "n-CC", userID, "uid-CC", 100, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))  // activity mar1
	backfillReadAt(t, pool)                                                                                // both after jan → stay unread.

	want := []string{"AA", "CC"} // AA(jun15) > CC(mar1); BB dropped (no unread).
	for _, limit := range []int{100, 1} {
		base := InZoneQuery{
			UserID: userID, Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 6000,
			Sort: SortRecentActivity, Unread: true, Limit: limit,
		}
		if got := pageAllFiltered(t, store, base); !reflect.DeepEqual(got, want) {
			t.Fatalf("recent-activity unread=true limit=%d: got %v, want %v", limit, got, want)
		}
	}
}

// TestPostgresStore_FindInZonePage_CursorFilterMismatch proves a cursor minted
// under one filter is rejected when replayed under another (status→other status,
// status→unread, filtered→unfiltered, unfiltered→filtered), and that a cursor
// replayed under the SAME filter keeps paging. Symmetric with the sort-mismatch
// guard: never a gapped or overlapping page.
func TestPostgresStore_FindInZonePage_CursorFilterMismatch(t *testing.T) {
	ctx := context.Background()
	store, pool := newActivityPGStore(t)
	statusFixture(t, store)
	const userID = "user-cursor"
	seedWatermark(t, pool, userID, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	seedNotification(t, pool, "n-P1", userID, "uid-P1", 100, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC))
	backfillReadAt(t, pool)

	permitted := InZoneQuery{
		Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 6000,
		Sort: SortNewest, Status: "Permitted", Limit: 1,
	}
	_, cursor, err := store.FindInZonePage(ctx, permitted)
	if err != nil {
		t.Fatalf("mint status=Permitted cursor: %v", err)
	}
	if cursor == "" {
		t.Fatal("expected a continuation cursor after a full filtered page")
	}

	// Replayed under a different status → mismatch.
	rejectedReplay := permitted
	rejectedReplay.Status, rejectedReplay.Cursor = "Rejected", cursor
	if _, _, err := store.FindInZonePage(ctx, rejectedReplay); !errors.Is(err, ErrCursorFilterMismatch) {
		t.Errorf("replay Permitted cursor under Rejected: got %v, want ErrCursorFilterMismatch", err)
	}

	// Replayed under the unread filter (status dropped) → mismatch.
	unreadReplay := InZoneQuery{
		UserID: userID, Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 6000,
		Sort: SortNewest, Unread: true, Cursor: cursor,
	}
	if _, _, err := store.FindInZonePage(ctx, unreadReplay); !errors.Is(err, ErrCursorFilterMismatch) {
		t.Errorf("replay Permitted cursor under unread: got %v, want ErrCursorFilterMismatch", err)
	}

	// Replayed unfiltered → mismatch.
	unfilteredReplay := InZoneQuery{
		Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 6000,
		Sort: SortNewest, Cursor: cursor,
	}
	if _, _, err := store.FindInZonePage(ctx, unfilteredReplay); !errors.Is(err, ErrCursorFilterMismatch) {
		t.Errorf("replay Permitted cursor unfiltered: got %v, want ErrCursorFilterMismatch", err)
	}

	// An unfiltered cursor replayed under a status filter → mismatch.
	_, plainCursor, err := store.FindInZonePage(ctx, InZoneQuery{
		Latitude: pgCentreLat, Longitude: pgCentreLon, RadiusMetres: 6000, Sort: SortNewest, Limit: 1,
	})
	if err != nil {
		t.Fatalf("mint unfiltered cursor: %v", err)
	}
	filteredReplay := permitted
	filteredReplay.Cursor = plainCursor
	if _, _, err := store.FindInZonePage(ctx, filteredReplay); !errors.Is(err, ErrCursorFilterMismatch) {
		t.Errorf("replay unfiltered cursor under Permitted: got %v, want ErrCursorFilterMismatch", err)
	}

	// Replayed under the SAME filter → keeps paging (no error, advances).
	sameReplay := permitted
	sameReplay.Cursor = cursor
	if _, _, err := store.FindInZonePage(ctx, sameReplay); err != nil {
		t.Errorf("replay Permitted cursor under Permitted: unexpected error %v", err)
	}
}

// TestPostgresStore_RecentNearby_And_NearestNearby distinguishes recency ordering
// (last_different DESC) from distance ordering (nearest first), and proves the
// radius filter and authority scope.
func TestPostgresStore_RecentNearby_And_NearestNearby(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	near := at(pgApp("NEAR", 100), 100)
	near.LastDifferent = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) // oldest
	far := at(pgApp("FAR", 100), 500)
	far.LastDifferent = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) // newest
	outside := at(pgApp("OUTSIDE", 100), 5000)                      // beyond 1 km
	otherAuth := at(pgApp("OTHER-AUTH", 200), 100)                  // wrong authority
	for _, a := range []PlanningApplication{near, far, outside, otherAuth} {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	recent, err := store.RecentNearby(ctx, "100", pgCentreLat, pgCentreLon, 1000, 10)
	if err != nil {
		t.Fatalf("RecentNearby: %v", err)
	}
	assertNames(t, appNames(recent), []string{"FAR", "NEAR"}) // recency DESC

	nearest, err := store.NearestNearby(ctx, "100", pgCentreLat, pgCentreLon, 1000, 10)
	if err != nil {
		t.Fatalf("NearestNearby: %v", err)
	}
	assertNames(t, appNames(nearest), []string{"NEAR", "FAR"}) // distance ASC

	capped, err := store.NearestNearby(ctx, "100", pgCentreLat, pgCentreLon, 1000, 1)
	if err != nil {
		t.Fatalf("NearestNearby capped: %v", err)
	}
	assertNames(t, appNames(capped), []string{"NEAR"})
}

// TestPostgresStore_BreakdownNearby returns the exact per-app_state counts over
// the in-radius, authority-scoped set, including the NULL bucket.
func TestPostgresStore_BreakdownNearby(t *testing.T) {
	ctx := context.Background()
	store := newAppPGStore(t)

	apps := []PlanningApplication{
		withState(at(pgApp("P1", 100), 100), pgPtr("Permitted")),
		withState(at(pgApp("P2", 100), 500), pgPtr("Permitted")),
		withState(at(pgApp("N1", 100), 300), nil),                   // NULL state, in radius
		withState(at(pgApp("OUT", 100), 5000), pgPtr("Permitted")),  // out of radius
		withState(at(pgApp("OTHER", 200), 100), pgPtr("Permitted")), // wrong authority
	}
	for _, a := range apps {
		if err := store.Upsert(ctx, a); err != nil {
			t.Fatalf("Upsert %s: %v", a.Name, err)
		}
	}

	got, err := store.BreakdownNearby(ctx, "100", pgCentreLat, pgCentreLon, 1000)
	if err != nil {
		t.Fatalf("BreakdownNearby: %v", err)
	}
	want := []StateCount{
		{AppState: pgPtr("Permitted"), Count: 2},
		{AppState: nil, Count: 1},
	}
	assertBreakdown(t, got, want)
}
