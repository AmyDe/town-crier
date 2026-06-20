package applications

import (
	"strconv"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// geoJSONPoint is the Cosmos GeoJSON projection of an application's coordinates:
// a Point with [longitude, latitude] order (GeoJSON convention), matching what
// Cosmos expects for ST_DISTANCE spatial queries.
type geoJSONPoint struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

// applicationDocument is the Cosmos persistence shape for a PlanningApplication
// in the Applications container. JSON tags use camelCase keys.
//
// Partition key: authorityCode (the AreaID as a string). Document id: the PlanIt
// case reference (Name). A point read is keyed on (authorityCode, name); upserts
// target the authorityCode partition.
type applicationDocument struct {
	ID            string              `json:"id"`
	AuthorityCode string              `json:"authorityCode"`
	PlanitName    string              `json:"planitName"`
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

// newApplicationDocument maps a domain snapshot to its Applications-container
// shape. The document id is the name and the partition key is the stringified
// area id.
func newApplicationDocument(a PlanningApplication) applicationDocument {
	return applicationDocument{
		ID:            a.Name,
		AuthorityCode: strconv.Itoa(a.AreaID),
		PlanitName:    a.Name,
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

// toDomain reconstitutes a domain snapshot from a stored document, unpacking the
// GeoJSON point back into flat longitude/latitude (coordinates[0]=lon, [1]=lat).
func (d applicationDocument) toDomain() PlanningApplication {
	lon, lat := coordsToLatLng(d.Location)
	return PlanningApplication{
		Name:          d.PlanitName,
		UID:           d.UID,
		AreaName:      d.AreaName,
		AreaID:        d.AreaID,
		Address:       d.Address,
		Postcode:      d.Postcode,
		Description:   d.Description,
		AppType:       d.AppType,
		AppState:      d.AppState,
		AppSize:       d.AppSize,
		StartDate:     platform.DateOnlyPtrToTime(d.StartDate),
		DecidedDate:   platform.DateOnlyPtrToTime(d.DecidedDate),
		ConsultedDate: platform.DateOnlyPtrToTime(d.ConsultedDate),
		Longitude:     lon,
		Latitude:      lat,
		URL:           d.URL,
		Link:          d.Link,
		LastDifferent: time.Time(d.LastDifferent),
	}
}

// newGeoPoint builds a GeoJSON point only when both coordinates are present;
// returns nil when either is absent.
func newGeoPoint(lon, lat *float64) *geoJSONPoint {
	if lon == nil || lat == nil {
		return nil
	}
	return &geoJSONPoint{Type: "Point", Coordinates: []float64{*lon, *lat}}
}

// coordsToLatLng unpacks a GeoJSON point into (longitude, latitude) pointers,
// returning (nil, nil) when the point is absent or malformed.
func coordsToLatLng(p *geoJSONPoint) (lon, lat *float64) {
	if p == nil || len(p.Coordinates) < 2 {
		return nil, nil
	}
	lo := p.Coordinates[0]
	la := p.Coordinates[1]
	return &lo, &la
}
