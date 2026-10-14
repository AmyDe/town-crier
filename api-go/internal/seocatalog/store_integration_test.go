//go:build integration

package seocatalog_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
	"github.com/AmyDe/town-crier/api-go/internal/seocatalog"
)

const (
	baseLat      = 51.5
	baseLng      = -0.12
	metresPerLat = 111_320.0
	districtType = "English District"
)

var baseTime = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

type fakeDirectory []authorities.Authority

func (d fakeDirectory) All() []authorities.Authority { return d }

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "seo_towns", "seo_town_applications", "seo_pages", "seo_redirects")
	return pool
}

func newStore(t *testing.T, pool *pgxpool.Pool, dir fakeDirectory, towns ...seocatalog.Town) *seocatalog.Store {
	t.Helper()
	raw, err := json.Marshal(towns)
	if err != nil {
		t.Fatalf("marshal towns: %v", err)
	}
	g, err := seocatalog.ParseGazetteer(raw)
	if err != nil {
		t.Fatalf("parse gazetteer: %v", err)
	}
	return seocatalog.NewStore(pool, g, dir)
}

func latNorth(metres float64) float64 { return baseLat + metres/metresPerLat }

func town(authorityID int, slug string, northMetres float64) seocatalog.Town {
	return seocatalog.Town{Slug: slug, Name: slug, Lat: latNorth(northMetres), Lng: baseLng, AuthorityID: authorityID, Population: 10000}
}

func district(id int, name string) authorities.Authority {
	return authorities.Authority{ID: id, Name: name, AreaType: districtType}
}

// seedApp inserts an application north of the base point; a nil northMetres
// stores a NULL location.
func seedApp(t *testing.T, pool *pgxpool.Pool, authorityID int, name string, northMetres *float64, lastDifferent time.Time) {
	t.Helper()
	var lat, lng any
	if northMetres != nil {
		lat, lng = latNorth(*northMetres), baseLng
	}
	_, err := pool.Exec(context.Background(), `
		INSERT INTO applications (planit_name, authority_code, uid, area_name, area_id, app_state, location, last_different)
		VALUES ($1, $2, $3, 'Area', $4, 'Undecided',
		        CASE WHEN $5::float8 IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($6::float8, $5::float8), 4326)::geography END, $7)`,
		name, strconv.Itoa(authorityID), "uid-"+name, authorityID, lat, lng, lastDifferent)
	if err != nil {
		t.Fatalf("seed application %s: %v", name, err)
	}
}

func seedDated(t *testing.T, pool *pgxpool.Pool, authorityID int, name string, northMetres float64, start, lastDifferent time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO applications (planit_name, authority_code, uid, area_name, area_id, app_state, location, start_date, last_different)
		VALUES ($1, $2, $3, 'Area', $4, 'Undecided', ST_SetSRID(ST_MakePoint($5::float8, $6::float8), 4326)::geography, $7, $8)`,
		name, strconv.Itoa(authorityID), "uid-"+name, authorityID, baseLng, latNorth(northMetres), start, lastDifferent)
	if err != nil {
		t.Fatalf("seed dated application %s: %v", name, err)
	}
}

func townList(t *testing.T, pool *pgxpool.Pool, path string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		"SELECT planit_name FROM seo_town_applications WHERE path = $1 ORDER BY rank", path)
	if err != nil {
		t.Fatalf("read town list %q: %v", path, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan town list: %v", err)
		}
		out = append(out, n)
	}
	return out
}

func at(m float64) *float64 { return &m }

func seedMany(t *testing.T, pool *pgxpool.Pool, authorityID int, prefix string, count int, northMetres float64) {
	t.Helper()
	for i := range count {
		seedApp(t, pool, authorityID, fmt.Sprintf("%s-%02d", prefix, i), at(northMetres), baseTime)
	}
}

func mustRebuild(t *testing.T, s *seocatalog.Store) seocatalog.CatalogSummary {
	t.Helper()
	sum, err := s.Rebuild(context.Background())
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	return sum
}

func pageExists(t *testing.T, pool *pgxpool.Pool, path string) bool {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM seo_pages WHERE path = $1", path).Scan(&n); err != nil {
		t.Fatalf("count page %q: %v", path, err)
	}
	return n == 1
}

func pagePaths(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT path FROM seo_pages ORDER BY sort_order")
	if err != nil {
		t.Fatalf("list pages: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatalf("scan page: %v", err)
		}
		out = append(out, p)
	}
	return out
}

func TestCatalog_AuthorityCoverageGate(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, fakeDirectory{district(1, "Enough"), district(2, "Too Few")})
	seedMany(t, pool, 1, "E", 10, 0)
	seedMany(t, pool, 2, "F", 9, 0)

	mustRebuild(t, s)

	if !pageExists(t, pool, "enough") {
		t.Error("authority with 10 applications was not published")
	}
	if pageExists(t, pool, "too-few") {
		t.Error("authority with 9 applications was published")
	}
	var total int
	if err := pool.QueryRow(context.Background(), "SELECT total FROM seo_pages WHERE path = 'enough'").Scan(&total); err != nil || total != 10 {
		t.Errorf("enough total = %d (err %v), want 10", total, err)
	}
}

func TestCatalog_TownCoverageGateUsesWholeRadius(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, fakeDirectory{district(1, "Shire")},
		town(1, "alpha", 0), town(1, "beta", 1000), town(1, "gamma", 20000))
	seedMany(t, pool, 1, "A", 6, 0)
	seedMany(t, pool, 1, "B", 6, 1000)
	seedMany(t, pool, 1, "G", 9, 20000)

	mustRebuild(t, s)

	for _, p := range []string{"shire/alpha", "shire/beta"} {
		if !pageExists(t, pool, p) {
			t.Errorf("page %q missing: whole-radius total (12) clears the gate", p)
		}
	}
	if pageExists(t, pool, "shire/gamma") {
		t.Error("town with 9 applications was published")
	}
	var total int
	if err := pool.QueryRow(context.Background(), "SELECT total FROM seo_pages WHERE path = 'shire/alpha'").Scan(&total); err != nil || total != 12 {
		t.Errorf("alpha total = %d (err %v), want 12", total, err)
	}
}

func TestCatalog_SameNameTownRedirects(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, fakeDirectory{district(1, "Wrexham")}, town(1, "wrexham", 0), town(1, "rhos", 20000))
	seedMany(t, pool, 1, "W", 12, 0)
	seedMany(t, pool, 1, "R", 12, 20000)

	mustRebuild(t, s)

	if pageExists(t, pool, "wrexham/wrexham") {
		t.Error("same-name town page was published")
	}
	if !pageExists(t, pool, "wrexham/rhos") {
		t.Error("distinct town page missing")
	}
	var target string
	if err := pool.QueryRow(context.Background(), "SELECT target FROM seo_redirects WHERE path = 'wrexham/wrexham'").Scan(&target); err != nil || target != "wrexham" {
		t.Errorf("redirect target = %q (err %v), want wrexham", target, err)
	}
}

func TestCatalog_DuplicateSlugSkipped(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, fakeDirectory{district(5, "Dupe"), district(6, "Dupe")})
	seedMany(t, pool, 5, "D5", 10, 0)
	seedMany(t, pool, 6, "D6", 10, 0)

	mustRebuild(t, s)

	var code string
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*), max(authority_code) FROM seo_pages WHERE kind = 'authority'").Scan(&n, &code); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 1 || code != "5" {
		t.Errorf("authority pages = %d (code %q), want 1 with code 5", n, code)
	}
}

type neighbourJSON struct {
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	AuthoritySlug string `json:"authoritySlug"`
}

func neighboursOf(t *testing.T, pool *pgxpool.Pool, path string) []neighbourJSON {
	t.Helper()
	var raw []byte
	if err := pool.QueryRow(context.Background(), "SELECT neighbours FROM seo_pages WHERE path = $1", path).Scan(&raw); err != nil {
		t.Fatalf("read neighbours for %q: %v", path, err)
	}
	var out []neighbourJSON
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode neighbours: %v", err)
	}
	return out
}

func TestCatalog_TownNeighboursEightCrossAuthority(t *testing.T) {
	pool := newPool(t)
	dir := fakeDirectory{district(1, "North Shire"), district(2, "South Shire")}
	var towns []seocatalog.Town
	for i := range 10 {
		authorityID := 1
		if i >= 5 {
			authorityID = 2
		}
		towns = append(towns, town(authorityID, fmt.Sprintf("t%d", i), float64(i)*10000))
		seedMany(t, pool, authorityID, fmt.Sprintf("T%d", i), 10, float64(i)*10000)
	}
	s := newStore(t, pool, dir, towns...)

	mustRebuild(t, s)

	got := neighboursOf(t, pool, "north-shire/t0")
	if len(got) != 8 {
		t.Fatalf("neighbours = %d, want 8", len(got))
	}
	for i, n := range got {
		wantSlug := fmt.Sprintf("t%d", i+1)
		wantAuthority := "north-shire"
		if i+1 >= 5 {
			wantAuthority = "south-shire"
		}
		if n.Slug != wantSlug || n.AuthoritySlug != wantAuthority {
			t.Errorf("neighbour %d = %s/%s, want %s/%s", i, n.AuthoritySlug, n.Slug, wantAuthority, wantSlug)
		}
	}
}

func TestCatalog_AuthorityNeighboursSixByTownCentroid(t *testing.T) {
	pool := newPool(t)
	var dir fakeDirectory
	var towns []seocatalog.Town
	for i := range 8 {
		id := i + 1
		dir = append(dir, district(id, fmt.Sprintf("Area %d", id)))
		towns = append(towns, town(id, fmt.Sprintf("town-%d", id), float64(i)*10000))
		seedMany(t, pool, id, fmt.Sprintf("A%d", id), 10, float64(i)*10000)
	}
	dir = append(dir, district(99, "No Towns"))
	seedMany(t, pool, 99, "NT", 10, 1000)
	s := newStore(t, pool, dir, towns...)

	mustRebuild(t, s)

	got := neighboursOf(t, pool, "area-1")
	if len(got) != 6 {
		t.Fatalf("neighbours = %d, want 6", len(got))
	}
	for i, n := range got {
		if want := fmt.Sprintf("area-%d", i+2); n.Slug != want {
			t.Errorf("neighbour %d = %q, want %q", i, n.Slug, want)
		}
	}
	if len(neighboursOf(t, pool, "no-towns")) != 0 {
		t.Error("authority without published towns has neighbours")
	}
}

func TestCatalog_ReplaceIsAtomicForTownLists(t *testing.T) {
	pool := newPool(t)
	dir := fakeDirectory{district(1, "Shire")}
	seedMany(t, pool, 1, "A", 12, 100)
	mustRebuild(t, newStore(t, pool, dir, town(1, "alpha", 0)))
	before := townList(t, pool, "shire/alpha")
	if len(before) != 12 {
		t.Fatalf("initial list = %d, want 12", len(before))
	}

	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		CREATE FUNCTION seo_town_applications_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'list insert blocked'; END $$;
		CREATE TRIGGER seo_town_applications_fail BEFORE INSERT ON seo_town_applications FOR EACH ROW EXECUTE FUNCTION seo_town_applications_fail()`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP TRIGGER IF EXISTS seo_town_applications_fail ON seo_town_applications; DROP FUNCTION IF EXISTS seo_town_applications_fail()")
	})

	seedMany(t, pool, 1, "B", 3, 100)
	if _, err := newStore(t, pool, dir, town(1, "alpha", 0)).Rebuild(ctx); err == nil {
		t.Fatal("Rebuild succeeded, want the blocked list insert to fail it")
	}

	if after := townList(t, pool, "shire/alpha"); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("list after failed rebuild = %v, want unchanged %v", after, before)
	}
}

func TestCatalog_ReplaceIsAtomic(t *testing.T) {
	pool := newPool(t)
	dir := fakeDirectory{district(1, "First"), district(2, "Second")}
	seedMany(t, pool, 1, "F", 10, 0)
	first := newStore(t, pool, dir)
	mustRebuild(t, first)
	before := pagePaths(t, pool)
	if !pageExists(t, pool, "first") {
		t.Fatal("first catalog missing authority page")
	}

	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		CREATE FUNCTION seo_redirects_fail() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'redirect insert blocked'; END $$;
		CREATE TRIGGER seo_redirects_fail BEFORE INSERT ON seo_redirects FOR EACH ROW EXECUTE FUNCTION seo_redirects_fail()`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP TRIGGER IF EXISTS seo_redirects_fail ON seo_redirects; DROP FUNCTION IF EXISTS seo_redirects_fail()")
	})

	seedMany(t, pool, 2, "S", 10, 0)
	seedMany(t, pool, 1, "W", 12, 20000)
	second := newStore(t, pool, fakeDirectory{district(1, "First"), district(2, "Second")}, town(1, "first", 20000))
	if _, err := second.Rebuild(ctx); err == nil {
		t.Fatal("Rebuild succeeded, want the blocked redirect insert to fail it")
	}

	after := pagePaths(t, pool)
	if fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("pages after failed rebuild = %v, want unchanged %v", after, before)
	}
	if pageExists(t, pool, "second") {
		t.Error("failed rebuild leaked the new authority page")
	}
}

func TestCatalog_TownsSlugCollisionKeepsPreviousCatalog(t *testing.T) {
	pool := newPool(t)
	seedMany(t, pool, 1, "F", 10, 0)
	seedMany(t, pool, 2, "T", 10, 0)
	mustRebuild(t, newStore(t, pool, fakeDirectory{district(1, "First")}))
	before := pagePaths(t, pool)

	colliding := newStore(t, pool, fakeDirectory{district(1, "First"), district(2, "Towns")})
	_, err := colliding.Rebuild(context.Background())

	if !errors.Is(err, seocatalog.ErrTownsSlugCollision) {
		t.Fatalf("Rebuild error = %v, want ErrTownsSlugCollision", err)
	}
	if after := pagePaths(t, pool); fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("pages after collision = %v, want unchanged %v", after, before)
	}
}

func TestCatalog_GazetteerSyncRemovesDeletedTowns(t *testing.T) {
	pool := newPool(t)
	dir := fakeDirectory{district(1, "Shire")}
	mustRebuild(t, newStore(t, pool, dir, town(1, "old-town", 0), town(1, "kept", 5000)))

	mustRebuild(t, newStore(t, pool, dir, town(1, "kept", 5000), town(1, "new-town", 9000)))

	rows, err := pool.Query(context.Background(), "SELECT slug FROM seo_towns ORDER BY slug")
	if err != nil {
		t.Fatalf("list towns: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, slug)
	}
	if fmt.Sprint(got) != "[kept new-town]" {
		t.Errorf("seo_towns = %v, want [kept new-town]", got)
	}
}

func TestCatalog_TownListNewestThirtyWithinRadius(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, fakeDirectory{district(1, "Shire")}, town(1, "alpha", 0))
	for i := range 35 {
		seedDated(t, pool, 1, fmt.Sprintf("IN-%02d", i), 100, baseTime.AddDate(0, 0, -i), baseTime)
	}
	seedDated(t, pool, 1, "OUT-NEWEST", 6000, baseTime.AddDate(0, 0, 10), baseTime)

	mustRebuild(t, s)

	got := townList(t, pool, "shire/alpha")
	if len(got) != 30 {
		t.Fatalf("list length = %d, want 30", len(got))
	}
	if got[0] != "IN-00" || got[29] != "IN-29" {
		t.Errorf("list runs %s..%s, want IN-00..IN-29 newest first", got[0], got[29])
	}
}

func TestCatalog_TownListOverlapsBetweenTowns(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, fakeDirectory{district(1, "Shire")}, town(1, "alpha", 0), town(1, "beta", 3000))
	seedMany(t, pool, 1, "SHARED", 12, 1500)

	mustRebuild(t, s)

	a, b := townList(t, pool, "shire/alpha"), townList(t, pool, "shire/beta")
	if len(a) != 12 || fmt.Sprint(a) != fmt.Sprint(b) {
		t.Errorf("alpha = %v, beta = %v, want the same 12 applications on both", a, b)
	}
}

func TestCatalog_TownListIgnoresOtherAuthority(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, fakeDirectory{district(1, "Shire"), district(2, "Other")}, town(1, "alpha", 0))
	seedMany(t, pool, 1, "OWN", 10, 100)
	seedMany(t, pool, 2, "FOREIGN", 5, 100)

	mustRebuild(t, s)

	for _, n := range townList(t, pool, "shire/alpha") {
		if len(n) >= 7 && n[:7] == "FOREIGN" {
			t.Errorf("list contains other-authority application %s", n)
		}
	}
	if got := len(townList(t, pool, "shire/alpha")); got != 10 {
		t.Errorf("list length = %d, want 10", got)
	}
}

func TestCatalog_TownListSkipsNullLocation(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, fakeDirectory{district(1, "Shire")}, town(1, "alpha", 0))
	seedMany(t, pool, 1, "LOC", 10, 100)
	seedApp(t, pool, 1, "NOLOC", nil, baseTime.Add(time.Hour))

	mustRebuild(t, s)

	for _, n := range townList(t, pool, "shire/alpha") {
		if n == "NOLOC" {
			t.Error("list contains an application with no location")
		}
	}
}

func TestCatalog_TownLastmodWholeRadius(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, fakeDirectory{district(1, "Shire")}, town(1, "alpha", 0))
	for i := range 31 {
		seedDated(t, pool, 1, fmt.Sprintf("N-%02d", i), 100, baseTime.AddDate(0, 0, 5-i), baseTime)
	}
	newest := baseTime.Add(48 * time.Hour)
	seedDated(t, pool, 1, "OLD-START-NEW-CHANGE", 200, baseTime.AddDate(-1, 0, 0), newest)

	mustRebuild(t, s)

	var lastmod *time.Time
	if err := pool.QueryRow(context.Background(), "SELECT lastmod FROM seo_pages WHERE path = 'shire/alpha'").Scan(&lastmod); err != nil {
		t.Fatalf("read lastmod: %v", err)
	}
	if lastmod == nil || !lastmod.Equal(newest) {
		t.Errorf("lastmod = %v, want %v from the whole 5,000 m set", lastmod, newest)
	}
}

func seedState(t *testing.T, pool *pgxpool.Pool, authorityID int, name string, northMetres float64, state *string, lastDifferent time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO applications (planit_name, authority_code, uid, area_name, area_id, app_state, location, last_different)
		VALUES ($1, $2, $3, 'Area', $4, $5, ST_SetSRID(ST_MakePoint($6::float8, $7::float8), 4326)::geography, $8)`,
		name, strconv.Itoa(authorityID), "uid-"+name, authorityID, state, baseLng, latNorth(northMetres), lastDifferent)
	if err != nil {
		t.Fatalf("seed application %s: %v", name, err)
	}
}

func seedStates(t *testing.T, pool *pgxpool.Pool, authorityID int, prefix string, northMetres float64, state *string, count int, lastDifferent time.Time) {
	t.Helper()
	for i := range count {
		seedState(t, pool, authorityID, fmt.Sprintf("%s-%02d", prefix, i), northMetres, state, lastDifferent)
	}
}

type townPage struct {
	total     int
	breakdown string
	lastmod   time.Time
}

func readTownPage(t *testing.T, pool *pgxpool.Pool, path string) townPage {
	t.Helper()
	var (
		p   townPage
		raw []byte
	)
	if err := pool.QueryRow(context.Background(),
		"SELECT total, status_breakdown, lastmod FROM seo_pages WHERE path = $1", path).Scan(&p.total, &raw, &p.lastmod); err != nil {
		t.Fatalf("read page %q: %v", path, err)
	}
	var counts []struct {
		AppState *string `json:"appState"`
		Count    int     `json:"count"`
	}
	if err := json.Unmarshal(raw, &counts); err != nil {
		t.Fatalf("decode breakdown for %q: %v", path, err)
	}
	for _, c := range counts {
		state := "<nil>"
		if c.AppState != nil {
			state = *c.AppState
		}
		p.breakdown += fmt.Sprintf("%s=%d ", state, c.Count)
	}
	return p
}

func TestCatalog_TownStatsPerTownRadiusAndAuthority(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, fakeDirectory{district(1, "Shire"), district(2, "Other")},
		town(1, "alpha", 0), town(1, "beta", 3000))
	approved, undecided, refused, withdrawn := "Approved", "Undecided", "Refused", "Withdrawn"

	seedStates(t, pool, 1, "BOTH-A", 1500, &approved, 4, baseTime)
	seedStates(t, pool, 1, "BOTH-U", 1500, &undecided, 3, baseTime)
	seedStates(t, pool, 1, "BOTH-N", 1500, nil, 2, baseTime)
	seedStates(t, pool, 1, "ALPHA-R", -2500, &refused, 2, baseTime.Add(72*time.Hour))
	seedStates(t, pool, 1, "BETA-W", 7500, &withdrawn, 2, baseTime.Add(48*time.Hour))
	seedState(t, pool, 1, "OUT-SOUTH", -5050, &approved, baseTime.Add(96*time.Hour))
	seedState(t, pool, 1, "OUT-NORTH", 8050, &approved, baseTime.Add(96*time.Hour))
	seedApp(t, pool, 1, "NO-LOCATION", nil, baseTime.Add(96*time.Hour))
	seedStates(t, pool, 2, "FOREIGN", 1500, &approved, 5, baseTime.Add(120*time.Hour))

	mustRebuild(t, s)

	tests := []struct {
		path string
		want townPage
	}{
		{"shire/alpha", townPage{11, "Approved=4 Undecided=3 Refused=2 <nil>=2 ", baseTime.Add(72 * time.Hour)}},
		{"shire/beta", townPage{11, "Approved=4 Undecided=3 Withdrawn=2 <nil>=2 ", baseTime.Add(48 * time.Hour)}},
	}
	for _, tc := range tests {
		got := readTownPage(t, pool, tc.path)
		if got.total != tc.want.total || got.breakdown != tc.want.breakdown || !got.lastmod.Equal(tc.want.lastmod) {
			t.Errorf("%s = %+v, want %+v", tc.path, got, tc.want)
		}
	}
}

func TestCatalog_TownStatsQueryScansApplicationsOnce(t *testing.T) {
	pool := newPool(t)
	ctx := context.Background()
	towns := make([]seocatalog.Town, 0, 20)
	for i := range 20 {
		towns = append(towns, town(1+i%4, fmt.Sprintf("t%02d", i), float64(i)*4000))
	}
	mustRebuild(t, newStore(t, pool, fakeDirectory{}, towns...))
	if _, err := pool.Exec(ctx, `
		INSERT INTO applications (planit_name, authority_code, uid, area_name, area_id, app_state, location, last_different)
		SELECT 'P-' || g, (1 + g % 4)::text, 'uid-P-' || g, 'Area', 1 + g % 4, 'Undecided',
		       ST_SetSRID(ST_MakePoint($1::float8, $2::float8 + (g % 800) * 0.001), 4326)::geography, $3
		FROM generate_series(1, 4000) g`, baseLng, baseLat, baseTime); err != nil {
		t.Fatalf("seed applications: %v", err)
	}
	if _, err := pool.Exec(ctx, "ANALYZE applications; ANALYZE seo_towns"); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	rows, err := pool.Query(ctx, "EXPLAIN "+seocatalog.TownStatsQuery, float64(seocatalog.TownRadiusMetres))
	if err != nil {
		t.Fatalf("explain: %v", err)
	}
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		plan.WriteString(line + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read plan: %v", err)
	}

	got := plan.String()
	if !strings.Contains(got, "Seq Scan on applications") ||
		strings.Contains(got, "Bitmap Heap Scan on applications") ||
		strings.Contains(got, "Index Scan using applications_location_gist") {
		t.Errorf("plan does not read applications once in a sequential scan:\n%s", got)
	}
}
