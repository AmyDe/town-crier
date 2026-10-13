//go:build integration

package seopage

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
	"github.com/AmyDe/town-crier/api-go/internal/seocatalog"
)

var baseTime = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

func newIntegrationStore(t *testing.T) (*PostgresStore, *pgxpool.Pool) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "seo_town_applications", "seo_pages", "seo_redirects")
	return NewPostgresStore(pool, applications.NewPostgresStore(pool)), pool
}

func seedPage(t *testing.T, pool *pgxpool.Pool, p seocatalog.Page, breakdown, children, neighbours string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO seo_pages (path, kind, authority_code, town_slug, display_name, authority_name, total,
		                       status_breakdown, children, neighbours, lat, lng, lastmod, sort_order)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, NULLIF($6, ''), $7, $8::jsonb, $9::jsonb, $10::jsonb, $11, $12, $13, $14)`,
		p.Path, p.Kind, p.AuthorityCode, p.TownSlug, p.DisplayName, p.AuthorityName, p.Total,
		breakdown, children, neighbours, p.Lat, p.Lng, p.Lastmod, p.SortOrder)
	if err != nil {
		t.Fatalf("seed page %q: %v", p.Path, err)
	}
}

func seedApplication(t *testing.T, pool *pgxpool.Pool, authorityCode, name string, decided *time.Time, lastDifferent time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO applications (planit_name, authority_code, uid, area_name, area_id, address, description, app_state, decided_date, location, last_different)
		VALUES ($1, $2, $3, 'Area', 1, 'Address '||$1, 'Description', 'Undecided', $4,
		        ST_SetSRID(ST_MakePoint(-0.12, 51.5), 4326)::geography, $5)`,
		name, authorityCode, "uid-"+name, decided, lastDifferent)
	if err != nil {
		t.Fatalf("seed application %s: %v", name, err)
	}
}

func listTown(t *testing.T, pool *pgxpool.Pool, path string, rank int, authorityCode, name string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO seo_town_applications (path, rank, authority_code, planit_name) VALUES ($1, $2, $3, $4)`,
		path, rank, authorityCode, name)
	if err != nil {
		t.Fatalf("list %s at %s#%d: %v", name, path, rank, err)
	}
}

func names(apps []applications.PlanningApplication) []string {
	out := make([]string, len(apps))
	for i := range apps {
		out[i] = apps[i].Name
	}
	return out
}

func TestPostgresStore_PageRoundTripsJSONColumns(t *testing.T) {
	store, pool := newIntegrationStore(t)
	lat, lng := 50.26, -5.05
	seedPage(t, pool, seocatalog.Page{
		Path: "cornwall/truro", Kind: seocatalog.KindTown, AuthorityCode: "52", TownSlug: "truro",
		DisplayName: "Truro", AuthorityName: "Cornwall", Total: 18, Lat: &lat, Lng: &lng, SortOrder: 3,
	},
		`[{"appState":"Permitted","count":10},{"appState":null,"count":2}]`,
		`[]`,
		`[{"name":"Falmouth","slug":"falmouth","authoritySlug":"cornwall","authorityName":"Cornwall"}]`)

	got, found, err := store.Page(context.Background(), "cornwall/truro")
	if err != nil || !found {
		t.Fatalf("Page = found %t, err %v", found, err)
	}
	permitted := "Permitted"
	want := seocatalog.Page{
		Path: "cornwall/truro", Kind: seocatalog.KindTown, AuthorityCode: "52", TownSlug: "truro",
		DisplayName: "Truro", AuthorityName: "Cornwall", Total: 18,
		StatusBreakdown: []seocatalog.StateCount{{AppState: &permitted, Count: 10}, {AppState: nil, Count: 2}},
		Children:        []seocatalog.TownLink{},
		Neighbours:      []seocatalog.Neighbour{{Name: "Falmouth", Slug: "falmouth", AuthoritySlug: "cornwall", AuthorityName: "Cornwall"}},
		Lat:             &lat, Lng: &lng, SortOrder: 3,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Page = %+v, want %+v", got, want)
	}
}

func TestPostgresStore_PageMissingAndNullableColumns(t *testing.T) {
	store, pool := newIntegrationStore(t)
	seedPage(t, pool, seocatalog.Page{Path: "", Kind: seocatalog.KindHub, DisplayName: "Planning applications"}, `[]`, `[]`, `[]`)

	hub, found, err := store.Page(context.Background(), "")
	if err != nil || !found {
		t.Fatalf("hub Page = found %t, err %v", found, err)
	}
	if hub.AuthorityCode != "" || hub.TownSlug != "" || hub.AuthorityName != "" || hub.Lat != nil {
		t.Errorf("NULL columns must read as zero values, got %+v", hub)
	}
	if _, found, err := store.Page(context.Background(), "nowhere"); err != nil || found {
		t.Errorf("missing Page = found %t, err %v, want not found", found, err)
	}
}

func TestPostgresStore_RedirectTarget(t *testing.T) {
	store, pool := newIntegrationStore(t)
	if _, err := pool.Exec(context.Background(), `INSERT INTO seo_redirects (path, target) VALUES ('wrexham/wrexham', 'wrexham')`); err != nil {
		t.Fatalf("seed redirect: %v", err)
	}
	target, found, err := store.RedirectTarget(context.Background(), "wrexham/wrexham")
	if err != nil || !found || target != "wrexham" {
		t.Fatalf("RedirectTarget = %q, %t, %v", target, found, err)
	}
	if _, found, err := store.RedirectTarget(context.Background(), "cornwall/truro"); err != nil || found {
		t.Fatalf("missing RedirectTarget = found %t, err %v", found, err)
	}
}

func TestPostgresStore_PagesFiltersKindInSortOrder(t *testing.T) {
	store, pool := newIntegrationStore(t)
	seedPage(t, pool, seocatalog.Page{Path: "b", Kind: seocatalog.KindAuthority, AuthorityCode: "2", DisplayName: "B", SortOrder: 2}, `[]`, `[]`, `[]`)
	seedPage(t, pool, seocatalog.Page{Path: "a", Kind: seocatalog.KindAuthority, AuthorityCode: "1", DisplayName: "A", SortOrder: 1}, `[]`, `[]`, `[]`)
	seedPage(t, pool, seocatalog.Page{Path: "a/x", Kind: seocatalog.KindTown, AuthorityCode: "1", TownSlug: "x", DisplayName: "X", SortOrder: 3}, `[]`, `[]`, `[]`)

	pages, err := store.Pages(context.Background(), seocatalog.KindAuthority)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	var paths []string
	for _, p := range pages {
		paths = append(paths, p.Path)
	}
	if !reflect.DeepEqual(paths, []string{"a", "b"}) {
		t.Fatalf("authority pages = %v, want [a b] in sort order", paths)
	}
}

func TestPostgresStore_AuthorityApplicationsOrderedByRealDateAndLimited(t *testing.T) {
	store, pool := newIntegrationStore(t)
	d := func(day int) *time.Time { v := time.Date(2026, 5, day, 0, 0, 0, 0, time.UTC); return &v }
	seedApplication(t, pool, "52", "old", d(1), baseTime.Add(5*time.Hour))
	seedApplication(t, pool, "52", "newest", d(20), baseTime)
	seedApplication(t, pool, "52", "middle", d(10), baseTime.Add(2*time.Hour))
	seedApplication(t, pool, "99", "other-authority", d(28), baseTime)

	apps, err := store.AuthorityApplications(context.Background(), "52", 2)
	if err != nil {
		t.Fatalf("AuthorityApplications: %v", err)
	}
	if got := names(apps); !reflect.DeepEqual(got, []string{"newest", "middle"}) {
		t.Fatalf("applications = %v, want [newest middle]", got)
	}
}

func TestPostgresStore_TownApplicationsFollowRankWithCurrentState(t *testing.T) {
	store, pool := newIntegrationStore(t)
	d := func(day int) *time.Time { v := time.Date(2026, 5, day, 0, 0, 0, 0, time.UTC); return &v }
	seedApplication(t, pool, "52", "second", d(2), baseTime)
	seedApplication(t, pool, "52", "first", d(25), baseTime)
	seedApplication(t, pool, "52", "other-town", d(28), baseTime)
	listTown(t, pool, "cornwall/truro", 2, "52", "second")
	listTown(t, pool, "cornwall/truro", 1, "52", "first")
	listTown(t, pool, "cornwall/falmouth", 1, "52", "other-town")
	listTown(t, pool, "cornwall/falmouth", 2, "52", "first")

	apps, err := store.TownApplications(context.Background(), "cornwall/truro")
	if err != nil {
		t.Fatalf("TownApplications: %v", err)
	}
	if got := names(apps); !reflect.DeepEqual(got, []string{"first", "second"}) {
		t.Fatalf("applications = %v, want [first second] in rank order", got)
	}
}

func TestPostgresStore_PageCarriesTownLastmod(t *testing.T) {
	store, pool := newIntegrationStore(t)
	lm := baseTime.Add(time.Hour)
	seedPage(t, pool, seocatalog.Page{Path: "cornwall/truro", Kind: seocatalog.KindTown, AuthorityCode: "52", TownSlug: "truro", DisplayName: "Truro", Lastmod: &lm}, `[]`, `[]`, `[]`)

	p, ok, err := store.Page(context.Background(), "cornwall/truro")
	if err != nil || !ok || p.Lastmod == nil || !p.Lastmod.Equal(lm) {
		t.Fatalf("Page = %+v, %v, %v, want lastmod %v", p, ok, err, lm)
	}
}

func TestPostgresStore_AuthorityLastmod(t *testing.T) {
	store, pool := newIntegrationStore(t)
	seedApplication(t, pool, "52", "a", nil, baseTime)
	seedApplication(t, pool, "52", "b", nil, baseTime.Add(48*time.Hour))

	got, err := store.AuthorityLastmod(context.Background(), "52")
	if err != nil || got == nil || !got.Equal(baseTime.Add(48*time.Hour)) {
		t.Fatalf("AuthorityLastmod = %v, %v, want %v", got, err, baseTime.Add(48*time.Hour))
	}
	empty, err := store.AuthorityLastmod(context.Background(), "404")
	if err != nil || empty != nil {
		t.Fatalf("empty AuthorityLastmod = %v, %v, want nil", empty, err)
	}
}

func TestPostgresStore_SitemapInSortOrderWithLastmods(t *testing.T) {
	store, pool := newIntegrationStore(t)
	seedPage(t, pool, seocatalog.Page{Path: "", Kind: seocatalog.KindHub, DisplayName: "Planning applications", SortOrder: 0}, `[]`, `[]`, `[]`)
	seedPage(t, pool, seocatalog.Page{Path: "cornwall", Kind: seocatalog.KindAuthority, AuthorityCode: "52", DisplayName: "Cornwall", SortOrder: 1}, `[]`, `[]`, `[]`)
	townLM := baseTime
	seedPage(t, pool, seocatalog.Page{Path: "cornwall/truro", Kind: seocatalog.KindTown, AuthorityCode: "52", TownSlug: "truro", DisplayName: "Truro", Lastmod: &townLM, SortOrder: 2}, `[]`, `[]`, `[]`)
	seedPage(t, pool, seocatalog.Page{Path: "empty", Kind: seocatalog.KindAuthority, AuthorityCode: "77", DisplayName: "Empty", SortOrder: 3}, `[]`, `[]`, `[]`)
	seedPage(t, pool, seocatalog.Page{Path: "towns", Kind: seocatalog.KindTowns, DisplayName: "Towns", SortOrder: 4}, `[]`, `[]`, `[]`)
	seedApplication(t, pool, "52", "a", nil, baseTime)
	seedApplication(t, pool, "52", "b", nil, baseTime.Add(24*time.Hour))

	entries, err := store.Sitemap(context.Background())
	if err != nil {
		t.Fatalf("Sitemap: %v", err)
	}
	type row struct {
		Path    string
		Lastmod *time.Time
	}
	var got []row
	for _, e := range entries {
		got = append(got, row{e.Path, e.Lastmod})
	}
	authorityLM := baseTime.Add(24 * time.Hour)
	want := []row{{"", nil}, {"cornwall", &authorityLM}, {"cornwall/truro", &townLM}, {"empty", nil}, {"towns", nil}}
	if len(got) != len(want) {
		t.Fatalf("Sitemap = %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Path != want[i].Path || (got[i].Lastmod == nil) != (want[i].Lastmod == nil) ||
			(want[i].Lastmod != nil && !got[i].Lastmod.Equal(*want[i].Lastmod)) {
			t.Errorf("entry %d = %v %v, want %v %v", i, got[i].Path, got[i].Lastmod, want[i].Path, want[i].Lastmod)
		}
	}
}
