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
// by the watch-zone create response and the public near-point endpoint. It is
// exactly Result without latestUnreadEvent — that field is a Result projection
// concern, absent from the domain shape.
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
	// AuthoritySlug is the URL slug of the application's authority, emitted ONLY
	// by the public near-point endpoint (internal/applications/nearpoint.go,
	// GH#879 Phase 1) so an anonymously-loaded application can build a share URL
	// or a by-slug detail fetch. It is DELIBERATELY omitempty and NOT set by
	// NearbyResultOf: the watch-zone create response
	// (watchzones.handler.create -> createResult.NearbyApplications), which also
	// embeds NearbyResult via NearbyResultOf, leaves it "" and so stays
	// byte-identical on the wire — mirroring Result.AuthoritySlug/ResultOf above.
	AuthoritySlug string `json:"authoritySlug,omitempty"`
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
