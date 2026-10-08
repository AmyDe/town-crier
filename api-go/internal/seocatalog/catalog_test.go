package seocatalog

import (
	"errors"
	"slices"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/authorities"
)

func stats(total int) AuthorityStats {
	return AuthorityStats{Total: total, Breakdown: []StateCount{{Count: total}}}
}

func auth(id int, name, areaType string) authorities.Authority {
	return authorities.Authority{ID: id, Name: name, AreaType: areaType}
}

func paths(c Catalog) []string {
	out := make([]string, len(c.Pages))
	for i, p := range c.Pages {
		out[i] = p.Path
	}
	return out
}

func TestBuildCatalog_SortOrderIsHubAuthoritiesTownsThenTownsIndex(t *testing.T) {
	t.Parallel()
	c, err := BuildCatalog(CatalogInput{
		Authorities: []authorities.Authority{
			auth(2, "Zeta", "English District"), auth(1, "Alpha", "English District"),
		},
		Towns: []Town{
			{Slug: "b-town", Name: "B Town", AuthorityID: 1, Lat: 51, Lng: 0},
			{Slug: "a-town", Name: "A Town", AuthorityID: 1, Lat: 51.1, Lng: 0},
		},
		AuthorityStats: map[string]AuthorityStats{"1": stats(20), "2": stats(10)},
		TownStats: map[TownKey]TownStats{
			{AuthorityCode: "1", Slug: "a-town"}: {Total: 10},
			{AuthorityCode: "1", Slug: "b-town"}: {Total: 10},
		},
	})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	want := []string{"", "alpha", "zeta", "alpha/a-town", "alpha/b-town", "towns"}
	if got := paths(c); !slices.Equal(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
	for i, p := range c.Pages {
		if p.SortOrder != i {
			t.Errorf("page %q sort order = %d, want %d", p.Path, p.SortOrder, i)
		}
	}
}

func TestBuildCatalog_NonQualifyingAreaTypeIsExcluded(t *testing.T) {
	t.Parallel()
	c, err := BuildCatalog(CatalogInput{
		Authorities:    []authorities.Authority{auth(1, "Greater Region", "English Region")},
		AuthorityStats: map[string]AuthorityStats{"1": stats(500)},
	})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	if got := paths(c); !slices.Equal(got, []string{"", "towns"}) {
		t.Errorf("paths = %v, want only hub and towns", got)
	}
}

func TestBuildCatalog_AuthorityPageCarriesChildrenSortedByName(t *testing.T) {
	t.Parallel()
	c, err := BuildCatalog(CatalogInput{
		Authorities: []authorities.Authority{auth(1, "Shire", "English District")},
		Towns: []Town{
			{Slug: "zed", Name: "Zed", AuthorityID: 1, Lat: 51, Lng: 0},
			{Slug: "abe", Name: "Abe", AuthorityID: 1, Lat: 51.2, Lng: 0},
		},
		AuthorityStats: map[string]AuthorityStats{"1": {Total: 30, AreaName: "Shire Council"}},
		TownStats: map[TownKey]TownStats{
			{AuthorityCode: "1", Slug: "zed"}: {Total: 10},
			{AuthorityCode: "1", Slug: "abe"}: {Total: 10},
		},
	})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	page := c.Pages[1]
	if page.DisplayName != "Shire Council" || page.AuthorityName != "Shire" {
		t.Errorf("names = (%q, %q), want (Shire Council, Shire)", page.DisplayName, page.AuthorityName)
	}
	want := []TownLink{{Name: "Abe", Slug: "abe"}, {Name: "Zed", Slug: "zed"}}
	if !slices.Equal(page.Children, want) {
		t.Errorf("children = %v, want %v", page.Children, want)
	}
	if page.Neighbours == nil || page.StatusBreakdown == nil {
		t.Error("JSON columns must be non-nil so they encode as []")
	}
}

func TestBuildCatalog_DuplicateTownPathKeepsFirst(t *testing.T) {
	t.Parallel()
	c, err := BuildCatalog(CatalogInput{
		Authorities: []authorities.Authority{auth(1, "Shire", "English District")},
		Towns: []Town{
			{Slug: "same", Name: "First", AuthorityID: 1, Lat: 51, Lng: 0},
			{Slug: "same", Name: "Second", AuthorityID: 1, Lat: 52, Lng: 0},
		},
		AuthorityStats: map[string]AuthorityStats{"1": stats(10)},
		TownStats:      map[TownKey]TownStats{{AuthorityCode: "1", Slug: "same"}: {Total: 10}},
	})
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}
	var town *Page
	for i := range c.Pages {
		if c.Pages[i].Kind == KindTown {
			town = &c.Pages[i]
		}
	}
	if town == nil || town.DisplayName != "First" {
		t.Errorf("town page = %+v, want the first record", town)
	}
}

func TestBuildCatalog_RejectsAuthorityOwningTownsSlug(t *testing.T) {
	t.Parallel()
	_, err := BuildCatalog(CatalogInput{Authorities: []authorities.Authority{auth(1, "Towns", "English Region")}})
	if !errors.Is(err, ErrTownsSlugCollision) {
		t.Errorf("error = %v, want ErrTownsSlugCollision", err)
	}
}

func TestBuildCatalog_UnknownTownAuthorityFails(t *testing.T) {
	t.Parallel()
	_, err := BuildCatalog(CatalogInput{
		Towns:     []Town{{Slug: "lost", Name: "Lost", AuthorityID: 9}},
		TownStats: map[TownKey]TownStats{{AuthorityCode: "9", Slug: "lost"}: {Total: 10}},
	})
	if err == nil {
		t.Error("want an error for a published town with no authority")
	}
}

func TestSortStateCounts_CountThenStateWithNilLast(t *testing.T) {
	t.Parallel()
	s := func(v string) *string { return &v }
	got := []StateCount{{nil, 5}, {s("Undecided"), 5}, {s("Permitted"), 5}, {s("Rejected"), 9}}
	sortStateCounts(got)
	want := []StateCount{{s("Rejected"), 9}, {s("Permitted"), 5}, {s("Undecided"), 5}, {nil, 5}}
	for i := range want {
		if got[i].Count != want[i].Count || (got[i].AppState == nil) != (want[i].AppState == nil) ||
			(got[i].AppState != nil && *got[i].AppState != *want[i].AppState) {
			t.Fatalf("order = %+v, want %+v", got, want)
		}
	}
}
