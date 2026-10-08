//go:build integration

package seocatalog_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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
	pgtest.Truncate(t, pool, "applications", "seo_towns", "seo_town_assignments", "seo_pages", "seo_redirects", "seo_state")
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

func at(m float64) *float64 { return &m }

func seedMany(t *testing.T, pool *pgxpool.Pool, authorityID int, prefix string, count int, northMetres float64) {
	t.Helper()
	for i := range count {
		seedApp(t, pool, authorityID, fmt.Sprintf("%s-%02d", prefix, i), at(northMetres), baseTime)
	}
}

func assignedTown(t *testing.T, pool *pgxpool.Pool, authorityID int, name string) (string, bool) {
	t.Helper()
	var slug string
	err := pool.QueryRow(context.Background(),
		"SELECT town_slug FROM seo_town_assignments WHERE authority_code = $1 AND planit_name = $2",
		strconv.Itoa(authorityID), name).Scan(&slug)
	if err != nil {
		return "", false
	}
	return slug, true
}

func mustAssign(t *testing.T, s *seocatalog.Store) seocatalog.AssignSummary {
	t.Helper()
	sum, err := s.Assign(context.Background())
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	return sum
}

func mustRebuild(t *testing.T, s *seocatalog.Store) seocatalog.CatalogSummary {
	t.Helper()
	sum, err := s.Rebuild(context.Background())
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	return sum
}

func TestAssign_NearestTownWithinRadius(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, nil, town(1, "far-town", 0), town(1, "near-town", 4000))
	seedApp(t, pool, 1, "APP-1", at(3000), baseTime)

	mustAssign(t, s)

	if got, ok := assignedTown(t, pool, 1, "APP-1"); !ok || got != "near-town" {
		t.Errorf("assigned town = %q (found %v), want near-town", got, ok)
	}
}

func TestAssign_TieGoesToLowerSlug(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, nil, town(1, "b-town", 1000), town(1, "a-town", 1000))
	seedApp(t, pool, 1, "APP-1", at(2000), baseTime)

	mustAssign(t, s)

	if got, _ := assignedTown(t, pool, 1, "APP-1"); got != "a-town" {
		t.Errorf("assigned town = %q, want a-town", got)
	}
}

func TestAssign_OutOfRangeHasNoAssignment(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, nil, town(1, "only-town", 0))
	seedApp(t, pool, 1, "APP-FAR", at(6000), baseTime)

	mustAssign(t, s)

	if got, ok := assignedTown(t, pool, 1, "APP-FAR"); ok {
		t.Errorf("assigned town = %q, want none", got)
	}
}

func TestAssign_OtherAuthorityTownIgnored(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, nil, town(1, "town-one", 0))
	seedApp(t, pool, 2, "APP-OTHER", at(100), baseTime)

	mustAssign(t, s)

	if got, ok := assignedTown(t, pool, 2, "APP-OTHER"); ok {
		t.Errorf("assigned town = %q, want none", got)
	}
}

func TestAssign_NullLocationRemovesAssignment(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, nil, town(1, "town-one", 0))
	seedApp(t, pool, 1, "APP-1", at(100), baseTime)
	mustAssign(t, s)
	if _, ok := assignedTown(t, pool, 1, "APP-1"); !ok {
		t.Fatal("expected an initial assignment")
	}

	if _, err := pool.Exec(context.Background(),
		"UPDATE applications SET location = NULL, last_different = $1 WHERE planit_name = 'APP-1'", baseTime.Add(time.Hour)); err != nil {
		t.Fatalf("null location: %v", err)
	}
	mustAssign(t, s)

	if got, ok := assignedTown(t, pool, 1, "APP-1"); ok {
		t.Errorf("assigned town = %q, want none after location removed", got)
	}
}

func TestAssign_ChangedLastDifferentReassigns(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, nil, town(1, "south-town", 0), town(1, "north-town", 4500))
	seedApp(t, pool, 1, "APP-1", at(100), baseTime)
	mustAssign(t, s)
	if got, _ := assignedTown(t, pool, 1, "APP-1"); got != "south-town" {
		t.Fatalf("initial town = %q, want south-town", got)
	}

	later := baseTime.Add(2 * time.Hour)
	if _, err := pool.Exec(context.Background(), `
		UPDATE applications SET last_different = $1,
		  location = ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography WHERE planit_name = 'APP-1'`,
		later, baseLng, latNorth(4400)); err != nil {
		t.Fatalf("move application: %v", err)
	}
	mustAssign(t, s)

	var slug string
	var lastDifferent time.Time
	if err := pool.QueryRow(context.Background(),
		"SELECT town_slug, last_different FROM seo_town_assignments WHERE planit_name = 'APP-1'").Scan(&slug, &lastDifferent); err != nil {
		t.Fatalf("read assignment: %v", err)
	}
	if slug != "north-town" || !lastDifferent.Equal(later) {
		t.Errorf("assignment = (%q, %v), want (north-town, %v)", slug, lastDifferent, later)
	}
}

func TestAssign_WatermarkSkipsUnchanged(t *testing.T) {
	pool := newPool(t)
	s := newStore(t, pool, nil, town(1, "town-one", 0))
	seedApp(t, pool, 1, "APP-1", at(100), baseTime)
	mustAssign(t, s)
	if _, err := pool.Exec(context.Background(), "DELETE FROM seo_town_assignments"); err != nil {
		t.Fatalf("delete assignments: %v", err)
	}

	sum := mustAssign(t, s)

	if _, ok := assignedTown(t, pool, 1, "APP-1"); ok {
		t.Error("unchanged application was reassigned despite the watermark")
	}
	if sum.Processed != 0 {
		t.Errorf("processed = %d, want 0", sum.Processed)
	}
}

func TestAssign_GazetteerHashChangeReassignsAll(t *testing.T) {
	pool := newPool(t)
	first := newStore(t, pool, nil, town(1, "old-town", 0))
	seedApp(t, pool, 1, "APP-1", at(100), baseTime)
	mustAssign(t, first)
	if got, _ := assignedTown(t, pool, 1, "APP-1"); got != "old-town" {
		t.Fatalf("initial town = %q, want old-town", got)
	}

	second := newStore(t, pool, nil, town(1, "new-town", 0))
	sum := mustAssign(t, second)

	if !sum.Reset {
		t.Error("Reset = false, want true for a changed gazetteer")
	}
	if got, _ := assignedTown(t, pool, 1, "APP-1"); got != "new-town" {
		t.Errorf("town after gazetteer change = %q, want new-town", got)
	}
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
	mustAssign(t, s)

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
