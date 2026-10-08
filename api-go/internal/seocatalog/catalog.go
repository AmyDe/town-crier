package seocatalog

import (
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/AmyDe/town-crier/api-go/internal/authorities"
)

// Page kinds stored in seo_pages.kind.
const (
	KindHub       = "hub"
	KindTowns     = "towns"
	KindAuthority = "authority"
	KindTown      = "town"
)

// Page-set parameters.
const (
	// CoverageThreshold is the minimum application count for a page to publish.
	CoverageThreshold = 10
	// AssignmentRadiusMetres is the town radius for assignment and for town
	// totals; it is the same for every town.
	AssignmentRadiusMetres = 5000
	// NearbyTownsCount is the neighbour count on a town page.
	NearbyTownsCount = 8
	// NeighbourAuthoritiesCount is the neighbour count on an authority page.
	NeighbourAuthoritiesCount = 6
	// TownsIndexSlug is the path segment owned by the towns index page.
	TownsIndexSlug = "towns"
)

// ErrTownsSlugCollision means an authority slug equals TownsIndexSlug, which the
// towns index page owns.
var ErrTownsSlugCollision = errors.New("authority slug collides with the towns index route")

// StateCount is one status_breakdown element: applications with AppState (nil
// for unknown). Elements are ordered by count descending, then state ascending,
// nil last on a count tie.
type StateCount struct {
	AppState *string `json:"appState"`
	Count    int     `json:"count"`
}

// TownLink is one children element of an authority page: a published town under
// it, sorted by name.
type TownLink struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// Neighbour is one neighbours element. Authority pages carry Name and Slug (the
// neighbour's page path is its Slug); town pages also carry AuthoritySlug and
// AuthorityName (the neighbour's page path is AuthoritySlug/Slug).
type Neighbour struct {
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	AuthoritySlug string `json:"authoritySlug,omitempty"`
	AuthorityName string `json:"authorityName,omitempty"`
}

// Page is one seo_pages row. Path is "" for the hub, "towns" for the towns
// index, "<authority>" for an authority and "<authority>/<town>" for a town.
// AuthorityCode and TownSlug are empty where not applicable (stored as NULL).
type Page struct {
	Path            string
	Kind            string
	AuthorityCode   string
	TownSlug        string
	DisplayName     string
	AuthorityName   string
	Total           int
	StatusBreakdown []StateCount
	Children        []TownLink
	Neighbours      []Neighbour
	Lat             *float64
	Lng             *float64
	SortOrder       int
}

// Redirect is one seo_redirects row: a suppressed same-name town path and the
// authority path it permanently redirects to.
type Redirect struct {
	Path   string
	Target string
}

// Catalog is the full recomputed page set.
type Catalog struct {
	Pages     []Page
	Redirects []Redirect
}

// AuthorityStats are the whole-authority aggregates for one authority code.
type AuthorityStats struct {
	Total     int
	AreaName  string
	Breakdown []StateCount
}

// TownKey identifies a gazetteer town.
type TownKey struct {
	AuthorityCode string
	Slug          string
}

// TownStats are the whole-radius aggregates for one town.
type TownStats struct {
	Total     int
	Breakdown []StateCount
}

// CatalogInput is everything BuildCatalog needs; it performs no I/O.
type CatalogInput struct {
	// Authorities is the full authority list in publish-priority order (the
	// first of two equal slugs wins).
	Authorities    []authorities.Authority
	Towns          []Town
	AuthorityStats map[string]AuthorityStats
	TownStats      map[TownKey]TownStats
}

type townRecord struct {
	town          Town
	authorityCode string
	authoritySlug string
	authorityName string
	path          string
	stats         TownStats
}

type authorityRecord struct {
	authority   authorities.Authority
	code        string
	slug        string
	displayName string
	stats       AuthorityStats
}

// BuildCatalog applies the page-set rules: an authority publishes when its area
// type qualifies and it has CoverageThreshold or more applications; a town
// publishes when it has that many within its whole radius, unless it shares its
// authority's name (then it redirects to the authority page).
func BuildCatalog(in CatalogInput) (Catalog, error) {
	for _, a := range in.Authorities {
		if authorities.Slugify(a.Name) == TownsIndexSlug {
			return Catalog{}, fmt.Errorf("authority %q: %w", a.Name, ErrTownsSlugCollision)
		}
	}

	byID := make(map[int]authorities.Authority, len(in.Authorities))
	for _, a := range in.Authorities {
		byID[a.ID] = a
	}

	townRecords, redirects, err := decideTowns(in, byID)
	if err != nil {
		return Catalog{}, err
	}
	authorityRecords := decideAuthorities(in)

	townsByAuthority := make(map[string][]TownLink)
	townPoints := make(map[string][]geoPoint)
	for _, r := range townRecords {
		townsByAuthority[r.authorityCode] = append(townsByAuthority[r.authorityCode], TownLink{Name: r.town.Name, Slug: r.town.Slug})
		townPoints[r.authorityCode] = append(townPoints[r.authorityCode], geoPoint{lat: r.town.Lat, lng: r.town.Lng})
	}

	authPages := authorityPages(authorityRecords, townsByAuthority, townPoints)
	tPages := townPages(townRecords)
	pages := make([]Page, 0, len(authPages)+len(tPages)+2)
	pages = append(pages, Page{Path: "", Kind: KindHub, DisplayName: "Planning applications", StatusBreakdown: []StateCount{}, Children: []TownLink{}, Neighbours: []Neighbour{}})
	pages = append(pages, authPages...)
	pages = append(pages, tPages...)
	pages = append(pages, Page{Path: TownsIndexSlug, Kind: KindTowns, DisplayName: "Towns", StatusBreakdown: []StateCount{}, Children: []TownLink{}, Neighbours: []Neighbour{}})
	for i := range pages {
		pages[i].SortOrder = i
	}
	return Catalog{Pages: pages, Redirects: redirects}, nil
}

func decideAuthorities(in CatalogInput) []authorityRecord {
	var out []authorityRecord
	seen := make(map[string]bool)
	for _, a := range in.Authorities {
		if !qualifyingAreaTypes[a.AreaType] {
			continue
		}
		code := strconv.Itoa(a.ID)
		stats := in.AuthorityStats[code]
		if stats.Total < CoverageThreshold {
			continue
		}
		slug := authorities.Slugify(a.Name)
		if seen[slug] {
			continue
		}
		seen[slug] = true
		display := stats.AreaName
		if display == "" {
			display = a.Name
		}
		out = append(out, authorityRecord{authority: a, code: code, slug: slug, displayName: display, stats: stats})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].slug < out[j].slug })
	return out
}

func decideTowns(in CatalogInput, byID map[int]authorities.Authority) ([]townRecord, []Redirect, error) {
	var records []townRecord
	var redirects []Redirect
	seenPaths := make(map[string]bool)
	seenRedirects := make(map[string]bool)
	for _, t := range in.Towns {
		code := strconv.Itoa(t.AuthorityID)
		stats := in.TownStats[TownKey{AuthorityCode: code, Slug: t.Slug}]
		if stats.Total < CoverageThreshold {
			continue
		}
		auth, ok := byID[t.AuthorityID]
		if !ok {
			return nil, nil, fmt.Errorf("town %q: no authority with id %d", t.Slug, t.AuthorityID)
		}
		authoritySlug := authorities.Slugify(auth.Name)
		path := authoritySlug + "/" + t.Slug
		if isSameNameAsAuthority(auth.Name, t.Slug) {
			if !seenRedirects[path] {
				seenRedirects[path] = true
				redirects = append(redirects, Redirect{Path: path, Target: authoritySlug})
			}
			continue
		}
		if seenPaths[path] {
			continue
		}
		seenPaths[path] = true
		records = append(records, townRecord{
			town: t, authorityCode: code, authoritySlug: authoritySlug,
			authorityName: auth.Name, path: path, stats: stats,
		})
	}
	sort.SliceStable(redirects, func(i, j int) bool { return redirects[i].Path < redirects[j].Path })
	return records, redirects, nil
}

func authorityPages(records []authorityRecord, townsByAuthority map[string][]TownLink, townPoints map[string][]geoPoint) []Page {
	centroids := make(map[string]geoPoint)
	for code, points := range townPoints {
		if c, ok := centroidOf(points); ok {
			centroids[code] = c
		}
	}

	var candidates []neighbourCandidate
	for i, r := range records {
		if c, ok := centroids[r.code]; ok {
			candidates = append(candidates, neighbourCandidate{point: c, identity: r.slug, tieKey: r.slug, index: i})
		}
	}

	pages := make([]Page, 0, len(records))
	for _, r := range records {
		children := append([]TownLink{}, townsByAuthority[r.code]...)
		sort.SliceStable(children, func(i, j int) bool {
			if c := authorities.CompareOrdinalIgnoreCase(children[i].Name, children[j].Name); c != 0 {
				return c < 0
			}
			return children[i].Slug < children[j].Slug
		})
		neighbours := []Neighbour{}
		if c, ok := centroids[r.code]; ok {
			origin := neighbourCandidate{point: c, identity: r.slug, tieKey: r.slug}
			for _, idx := range nearestK(origin, candidates, NeighbourAuthoritiesCount) {
				n := records[idx]
				neighbours = append(neighbours, Neighbour{Name: n.displayName, Slug: n.slug})
			}
		}
		pages = append(pages, Page{
			Path: r.slug, Kind: KindAuthority, AuthorityCode: r.code,
			DisplayName: r.displayName, AuthorityName: r.authority.Name,
			Total: r.stats.Total, StatusBreakdown: nonNilBreakdown(r.stats.Breakdown),
			Children: children, Neighbours: neighbours,
		})
	}
	return pages
}

func townPages(records []townRecord) []Page {
	sorted := append([]townRecord(nil), records...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].path < sorted[j].path })

	candidates := make([]neighbourCandidate, len(sorted))
	for i, r := range sorted {
		candidates[i] = neighbourCandidate{
			point:    geoPoint{lat: r.town.Lat, lng: r.town.Lng},
			identity: r.path, tieKey: r.town.Slug, index: i,
		}
	}

	pages := make([]Page, 0, len(sorted))
	for i, r := range sorted {
		neighbours := []Neighbour{}
		for _, idx := range nearestK(candidates[i], candidates, NearbyTownsCount) {
			n := sorted[idx]
			neighbours = append(neighbours, Neighbour{
				Name: n.town.Name, Slug: n.town.Slug,
				AuthoritySlug: n.authoritySlug, AuthorityName: n.authorityName,
			})
		}
		lat, lng := r.town.Lat, r.town.Lng
		pages = append(pages, Page{
			Path: r.path, Kind: KindTown, AuthorityCode: r.authorityCode, TownSlug: r.town.Slug,
			DisplayName: r.town.Name, AuthorityName: r.authorityName,
			Total: r.stats.Total, StatusBreakdown: nonNilBreakdown(r.stats.Breakdown),
			Children: []TownLink{}, Neighbours: neighbours, Lat: &lat, Lng: &lng,
		})
	}
	return pages
}

func nonNilBreakdown(b []StateCount) []StateCount {
	if b == nil {
		return []StateCount{}
	}
	return b
}

// sortStateCounts orders a breakdown by count descending, then state ascending,
// with the nil state last on a count tie.
func sortStateCounts(out []StateCount) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].AppState == nil {
			return false
		}
		if out[j].AppState == nil {
			return true
		}
		return *out[i].AppState < *out[j].AppState
	})
}

var qualifyingAreaTypes = map[string]bool{
	"English District":          true,
	"English Unitary Authority": true,
	"Council District":          true,
	"Metropolitan Borough":      true,
	"London Borough":            true,
	"Scottish Council":          true,
	"Welsh Principal Area":      true,
	"National Park":             true,
	"Northern Ireland District": true,
}
