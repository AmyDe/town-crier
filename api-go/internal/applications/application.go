// Package applications owns the master planning-application feature: the domain
// snapshot, the Cosmos store over the Applications container (point read by
// authority + name), and the read endpoints (GET /v1/applications/{authorityCode}/{name}
// and GET /v1/me/application-authorities). It mirrors the .NET
// TownCrier.{Domain,Application,Infrastructure}.PlanningApplications slices
// (GH#418 iteration 6).
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
// pointers so an absent value serialises as JSON null, matching the .NET
// nullable properties. Coordinates are carried flat here; the Cosmos document
// projects them into a GeoJSON point.
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

// CanonicalUID is the server-derived identity "{AreaID}/{Name}". PlanIt case
// references are only unique within a council, so the authority must be part of
// the key. This is deliberately independent of the raw UID field — a client may
// send a stale-format uid, but two saves of the same (AreaID, Name) always
// produce the same canonical uid, keeping the saved-application doc id stable
// and re-saves idempotent. Mirrors .NET PlanningApplication.CanonicalUid.
func (a PlanningApplication) CanonicalUID() string {
	return strconv.Itoa(a.AreaID) + "/" + a.Name
}
