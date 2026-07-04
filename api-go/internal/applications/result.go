package applications

import (
	"encoding/json"
	"sort"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// Result is the wire shape of a planning application returned by the API.
// Coordinates are flat (the GeoJSON projection is a Cosmos storage concern
// only). latestUnreadEvent is always null on these endpoints; a nil
// json.RawMessage marshals to explicit JSON null.
type Result struct {
	Name              string              `json:"name"`
	UID               string              `json:"uid"`
	AreaName          string              `json:"areaName"`
	AreaID            int                 `json:"areaId"`
	Address           string              `json:"address"`
	Postcode          *string             `json:"postcode"`
	Description       string              `json:"description"`
	AppType           *string             `json:"appType"`
	AppState          *string             `json:"appState"`
	AppSize           *string             `json:"appSize"`
	StartDate         *platform.DateOnly  `json:"startDate"`
	DecidedDate       *platform.DateOnly  `json:"decidedDate"`
	ConsultedDate     *platform.DateOnly  `json:"consultedDate"`
	Longitude         *float64            `json:"longitude"`
	Latitude          *float64            `json:"latitude"`
	URL               *string             `json:"url"`
	Link              *string             `json:"link"`
	LastDifferent     platform.DotNetTime `json:"lastDifferent"`
	LatestUnreadEvent json.RawMessage     `json:"latestUnreadEvent"`
	// AuthoritySlug is the URL slug of the application's authority, emitted by the
	// single-application detail (by-id) and by-slug handlers so clients can build
	// share URLs (#738). It is DELIBERATELY omitempty and NOT set by ResultOf: the
	// savedapplications and watchzones responses that embed Result via ResultOf
	// leave it "" and so stay byte-identical on the wire.
	AuthoritySlug string `json:"authoritySlug,omitempty"`
}

// ResultOf maps a domain snapshot to its wire shape. It deliberately leaves
// AuthoritySlug empty (see the field comment); only the detail and by-slug
// handlers set it.
func ResultOf(a PlanningApplication) Result {
	return Result{
		Name:          a.Name,
		UID:           a.UID,
		AreaName:      a.AreaName,
		AreaID:        a.AreaID,
		Address:       a.Address,
		Postcode:      a.Postcode,
		Description:   a.Description,
		AppType:       a.AppType,
		AppState:      a.AppState,
		AppSize:       a.AppSize,
		StartDate:     platform.DateOnlyPtr(a.StartDate),
		DecidedDate:   platform.DateOnlyPtr(a.DecidedDate),
		ConsultedDate: platform.DateOnlyPtr(a.ConsultedDate),
		Longitude:     a.Longitude,
		Latitude:      a.Latitude,
		URL:           a.URL,
		Link:          a.Link,
		LastDifferent: platform.DotNetTime(a.LastDifferent),
	}
}

// NearbyResult is the wire shape of a raw domain PlanningApplication, as emitted
// by the watch-zone create response. It is exactly Result without
// latestUnreadEvent — that field is a Result projection concern, absent from
// the domain shape.
type NearbyResult struct {
	Name          string              `json:"name"`
	UID           string              `json:"uid"`
	AreaName      string              `json:"areaName"`
	AreaID        int                 `json:"areaId"`
	Address       string              `json:"address"`
	Postcode      *string             `json:"postcode"`
	Description   string              `json:"description"`
	AppType       *string             `json:"appType"`
	AppState      *string             `json:"appState"`
	AppSize       *string             `json:"appSize"`
	StartDate     *platform.DateOnly  `json:"startDate"`
	DecidedDate   *platform.DateOnly  `json:"decidedDate"`
	ConsultedDate *platform.DateOnly  `json:"consultedDate"`
	Longitude     *float64            `json:"longitude"`
	Latitude      *float64            `json:"latitude"`
	URL           *string             `json:"url"`
	Link          *string             `json:"link"`
	LastDifferent platform.DotNetTime `json:"lastDifferent"`
}

// RecentByAuthorityResult is the wire shape of the build-time SEO endpoint
// GET /v1/authorities/{id}/applications. Applications is always a non-null array
// (at most the request's limit). Total is the EXACT whole-partition total: the
// sum of the StatusBreakdown buckets, so the rendered "tracking N applications"
// lead line always equals the breakdown. StatusBreakdown is the per-appState
// distribution over the WHOLE authority partition (its denominator is the whole
// partition, not the bounded read), computed by an index-served GROUP BY, always
// a non-null array.
type RecentByAuthorityResult struct {
	AuthorityID     int                 `json:"authorityId"`
	AreaName        string              `json:"areaName"`
	Applications    []RecentApplication `json:"applications"`
	Total           int                 `json:"total"`
	StatusBreakdown []StateCount        `json:"statusBreakdown"`
}

// RecentNearbyResult is the wire shape of the build-time town-level SEO endpoint
// GET /v1/applications/near. It mirrors RecentByAuthorityResult but echoes the
// effective (post-clamp) query point and radius instead of an area name, so the
// town prerender can label and cache the page by its centroid. Applications is
// always a non-null array (at most the request's limit). Total is the EXACT
// whole-in-radius total: the sum of the StatusBreakdown buckets, so the rendered
// "tracking N applications" lead line always equals the breakdown. StatusBreakdown
// is the per-appState distribution over the WHOLE in-radius set (its denominator
// is the whole in-radius set, not the bounded read), computed by an index-served
// spatial GROUP BY, always a non-null array.
type RecentNearbyResult struct {
	AuthorityID     int                 `json:"authorityId"`
	Lat             float64             `json:"lat"`
	Lng             float64             `json:"lng"`
	Radius          float64             `json:"radius"`
	Applications    []RecentApplication `json:"applications"`
	Total           int                 `json:"total"`
	StatusBreakdown []StateCount        `json:"statusBreakdown"`
}

// RecentApplication is the slim, render-only projection of a planning application
// for an SEO page: just the fields the static page needs. Coordinates, area
// identity, and the unread-event projection are deliberately omitted.
// lastDifferent is the DESC sort key of the bounded read, carried so the web card
// can show a "Last updated" date that matches the list order; it is non-pointer
// because the domain always carries it. decidedDate (#819 decision 5) is the
// real-world "Decided" date shown alongside startDate's "Started" on the card;
// nil while the application is still undecided.
type RecentApplication struct {
	UID           string              `json:"uid"`
	Name          string              `json:"name"`
	Address       string              `json:"address"`
	Description   string              `json:"description"`
	AppState      *string             `json:"appState"`
	StartDate     *platform.DateOnly  `json:"startDate"`
	DecidedDate   *platform.DateOnly  `json:"decidedDate"`
	LastDifferent platform.DotNetTime `json:"lastDifferent"`
	Link          *string             `json:"link"`
	URL           *string             `json:"url"`
}

// RecentApplicationOf maps a domain snapshot to its slim SEO wire shape.
func RecentApplicationOf(a PlanningApplication) RecentApplication {
	return RecentApplication{
		UID:           a.UID,
		Name:          a.Name,
		Address:       a.Address,
		Description:   a.Description,
		AppState:      a.AppState,
		StartDate:     platform.DateOnlyPtr(a.StartDate),
		DecidedDate:   platform.DateOnlyPtr(a.DecidedDate),
		LastDifferent: platform.DotNetTime(a.LastDifferent),
		Link:          a.Link,
		URL:           a.URL,
	}
}

// StateCount is one row of an appState breakdown: a nullable raw appState and how
// many applications carried it. The appState is the RAW PlanIt value (nil when
// absent), not a resident-facing label — the web owns that mapping.
type StateCount struct {
	AppState *string `json:"appState"`
	Count    int     `json:"count"`
}

// sortStateCounts orders a breakdown deterministically in place: count DESC, then
// raw appState ASC, with the nil-appState bucket sorting last on a count tie. It
// is the single comparator shared by CosmosStore.BreakdownByAuthority (over a
// whole-partition GROUP BY) and CosmosStore.BreakdownNearby (over a whole-in-radius
// spatial GROUP BY).
func sortStateCounts(out []StateCount) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count // count DESC
		}
		// nil sorts last on a count tie.
		if out[i].AppState == nil {
			return false
		}
		if out[j].AppState == nil {
			return true
		}
		return *out[i].AppState < *out[j].AppState // appState ASC
	})
}

// NearbyResultOf maps a domain snapshot to the raw-domain wire shape.
func NearbyResultOf(a PlanningApplication) NearbyResult {
	return NearbyResult{
		Name:          a.Name,
		UID:           a.UID,
		AreaName:      a.AreaName,
		AreaID:        a.AreaID,
		Address:       a.Address,
		Postcode:      a.Postcode,
		Description:   a.Description,
		AppType:       a.AppType,
		AppState:      a.AppState,
		AppSize:       a.AppSize,
		StartDate:     platform.DateOnlyPtr(a.StartDate),
		DecidedDate:   platform.DateOnlyPtr(a.DecidedDate),
		ConsultedDate: platform.DateOnlyPtr(a.ConsultedDate),
		Longitude:     a.Longitude,
		Latitude:      a.Latitude,
		URL:           a.URL,
		Link:          a.Link,
		LastDifferent: platform.DotNetTime(a.LastDifferent),
	}
}
