package applications

import (
	"encoding/json"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// applicationResult mirrors .NET PlanningApplicationResult — the wire shape of a
// planning application. Coordinates are flat (the GeoJSON projection is a Cosmos
// storage concern only). latestUnreadEvent is always null on these endpoints
// (the .NET ToResult never populates it); a nil json.RawMessage marshals to the
// explicit null .NET emits.
type applicationResult struct {
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

// resultFrom maps a domain snapshot to its wire shape, mirroring .NET
// GetApplicationByUidQueryHandler.ToResult.
func resultFrom(a PlanningApplication) applicationResult {
	return applicationResult{
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
