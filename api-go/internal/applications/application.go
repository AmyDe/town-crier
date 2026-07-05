// Package applications owns the master planning-application feature: the domain
// snapshot, the Cosmos store over the Applications container (point read by
// authority + name), and the read endpoints (GET /v1/applications/{authorityCode}/{name}
// and GET /v1/me/application-authorities).
//
// A PlanningApplication is a snapshot of a PlanIt case: a plain data carrier
// (the values come from an external provider, validated at the HTTP boundary),
// not a rich aggregate.
package applications

import (
	"strconv"
	"time"
)

// PlanningApplication is the planning-application snapshot. Nullable fields are
// pointers so an absent value serialises as JSON null. Coordinates are carried
// flat here; the Cosmos document projects them into a GeoJSON point.
type PlanningApplication struct {
	Name          string
	UID           string
	AreaName      string
	AreaID        int
	Address       string
	Postcode      *string
	Description   string
	AppType       *string
	AppState      *string
	AppSize       *string
	StartDate     *time.Time
	DecidedDate   *time.Time
	ConsultedDate *time.Time
	Longitude     *float64
	Latitude      *float64
	URL           *string
	Link          *string
	LastDifferent time.Time
}

// TownCentroid is a validated WGS84 point plus its OWN safety radius: one
// gazetteer town's centroid, used by RecentNearestTown's query-time Voronoi
// partition (#819 decisions 2-3). The town whose read is being served passes
// its own (lat, lng, radius) as separate parameters; every OTHER gazetteer
// town in the same authority travels as a TownCentroid "sibling", competing on
// nearest-centroid assignment. Carrying a radius per town (not one radius
// shared by every town) is what makes the in-range-nearest rule possible: a
// farther town with a wider catchment can still claim an application that a
// nearer but narrower-catchment neighbour cannot reach.
type TownCentroid struct {
	Lat          float64
	Lng          float64
	RadiusMetres float64
}

// CanonicalUID is the server-derived identity "{AreaID}/{Name}". PlanIt case
// references are only unique within a council, so the authority must be part of
// the key. This is deliberately independent of the raw UID field — a client may
// send a stale-format uid, but two saves of the same (AreaID, Name) always
// produce the same canonical uid, keeping the saved-application doc id stable
// and re-saves idempotent.
func (a PlanningApplication) CanonicalUID() string {
	return strconv.Itoa(a.AreaID) + "/" + a.Name
}

// HasSameBusinessFieldsAs reports whether every business-material field matches
// other, ignoring LastDifferent. The poll cycle uses it to skip a redundant
// upsert when PlanIt re-emits an application with only a bumped LastDifferent
// timestamp — the load-bearing reindex-flood guard.
func (a PlanningApplication) HasSameBusinessFieldsAs(other PlanningApplication) bool {
	return a.Name == other.Name &&
		a.UID == other.UID &&
		a.AreaName == other.AreaName &&
		a.AreaID == other.AreaID &&
		a.Address == other.Address &&
		eqStrPtr(a.Postcode, other.Postcode) &&
		a.Description == other.Description &&
		eqStrPtr(a.AppType, other.AppType) &&
		eqStrPtr(a.AppState, other.AppState) &&
		eqStrPtr(a.AppSize, other.AppSize) &&
		eqTimePtr(a.StartDate, other.StartDate) &&
		eqTimePtr(a.DecidedDate, other.DecidedDate) &&
		eqTimePtr(a.ConsultedDate, other.ConsultedDate) &&
		eqFloatPtr(a.Longitude, other.Longitude) &&
		eqFloatPtr(a.Latitude, other.Latitude) &&
		eqStrPtr(a.URL, other.URL) &&
		eqStrPtr(a.Link, other.Link)
}

// eqStrPtr reports whether two optional strings are equal (both nil, or both
// set to the same value).
func eqStrPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// eqFloatPtr reports whether two optional floats are equal.
func eqFloatPtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// eqTimePtr reports whether two optional date values are equal.
func eqTimePtr(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}
