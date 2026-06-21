package applications

import (
	"encoding/json"

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
}

// ResultOf maps a domain snapshot to its wire shape.
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
// (at most the request's limit). Total is the count from the single bounded read;
// TotalCapped reports that the read hit cap (so the prerender can render "200+").
type RecentByAuthorityResult struct {
	AuthorityID  int                 `json:"authorityId"`
	AreaName     string              `json:"areaName"`
	Applications []RecentApplication `json:"applications"`
	Total        int                 `json:"total"`
	TotalCapped  bool                `json:"totalCapped"`
}

// RecentNearbyResult is the wire shape of the build-time town-level SEO endpoint
// GET /v1/applications/near. It mirrors RecentByAuthorityResult but echoes the
// effective (post-clamp) query point and radius instead of an area name, so the
// town prerender can label and cache the page by its centroid. Applications is
// always a non-null array (at most the request's limit); Total is the count from
// the single bounded read; TotalCapped reports that the read hit cap.
type RecentNearbyResult struct {
	AuthorityID  int                 `json:"authorityId"`
	Lat          float64             `json:"lat"`
	Lng          float64             `json:"lng"`
	Radius       float64             `json:"radius"`
	Applications []RecentApplication `json:"applications"`
	Total        int                 `json:"total"`
	TotalCapped  bool                `json:"totalCapped"`
}

// RecentApplication is the slim, render-only projection of a planning application
// for an SEO page: just the fields the static page needs. Coordinates, area
// identity, and the unread-event projection are deliberately omitted.
type RecentApplication struct {
	UID         string             `json:"uid"`
	Name        string             `json:"name"`
	Address     string             `json:"address"`
	Description string             `json:"description"`
	AppState    *string            `json:"appState"`
	StartDate   *platform.DateOnly `json:"startDate"`
	Link        *string            `json:"link"`
	URL         *string            `json:"url"`
}

// RecentApplicationOf maps a domain snapshot to its slim SEO wire shape.
func RecentApplicationOf(a PlanningApplication) RecentApplication {
	return RecentApplication{
		UID:         a.UID,
		Name:        a.Name,
		Address:     a.Address,
		Description: a.Description,
		AppState:    a.AppState,
		StartDate:   platform.DateOnlyPtr(a.StartDate),
		Link:        a.Link,
		URL:         a.URL,
	}
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
