package applications

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// SnapshotDocument is the embedded planning-application snapshot stored inline on
// a saved-application document. It carries the same fields as the master
// applicationDocument minus the container identity (id / authorityCode /
// planitName) — the case reference is the "name" key here. Mirrors .NET
// SavedApplicationSnapshotDocument. The applications package owns it because it
// is a PlanningApplication wire projection; the savedapplications package embeds
// it without duplicating the GeoJSON / date mapping.
type SnapshotDocument struct {
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
	Location      *geoJSONPoint       `json:"location"`
	URL           *string             `json:"url"`
	Link          *string             `json:"link"`
	LastDifferent platform.DotNetTime `json:"lastDifferent"`
}

// NewSnapshotDocument projects a domain snapshot into the embedded shape.
func NewSnapshotDocument(a PlanningApplication) SnapshotDocument {
	return SnapshotDocument{
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
		Location:      newGeoPoint(a.Longitude, a.Latitude),
		URL:           a.URL,
		Link:          a.Link,
		LastDifferent: platform.DotNetTime(a.LastDifferent),
	}
}

// ToDomain reconstitutes the embedded snapshot into a domain application.
func (s SnapshotDocument) ToDomain() PlanningApplication {
	lon, lat := coordsToLatLng(s.Location)
	return PlanningApplication{
		Name:          s.Name,
		UID:           s.UID,
		AreaName:      s.AreaName,
		AreaID:        s.AreaID,
		Address:       s.Address,
		Postcode:      s.Postcode,
		Description:   s.Description,
		AppType:       s.AppType,
		AppState:      s.AppState,
		AppSize:       s.AppSize,
		StartDate:     platform.DateOnlyPtrToTime(s.StartDate),
		DecidedDate:   platform.DateOnlyPtrToTime(s.DecidedDate),
		ConsultedDate: platform.DateOnlyPtrToTime(s.ConsultedDate),
		Longitude:     lon,
		Latitude:      lat,
		URL:           s.URL,
		Link:          s.Link,
		LastDifferent: time.Time(s.LastDifferent),
	}
}
