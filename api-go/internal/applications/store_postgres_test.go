//go:build integration

package applications

import (
	"context"
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
func newAppPGStore(t *testing.T) (*PostgresStore, *pgxpool.Pool) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	return NewPostgresStore(pool), pool
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
	store, _ := newAppPGStore(t)

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
	store, _ := newAppPGStore(t)

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
	store, _ := newAppPGStore(t)

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
	store, _ := newAppPGStore(t)

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
	store, _ := newAppPGStore(t)

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
	store, _ := newAppPGStore(t)

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
	store, _ := newAppPGStore(t)

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
	store, _ := newAppPGStore(t)

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
	store, _ := newAppPGStore(t)

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
	store, _ := newAppPGStore(t)

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

// TestPostgresStore_RecentNearby_And_NearestNearby distinguishes recency ordering
// (last_different DESC) from distance ordering (nearest first), and proves the
// radius filter and authority scope.
func TestPostgresStore_RecentNearby_And_NearestNearby(t *testing.T) {
	ctx := context.Background()
	store, _ := newAppPGStore(t)

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
	store, _ := newAppPGStore(t)

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
