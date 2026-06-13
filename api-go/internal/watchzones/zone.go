// Package watchzones owns the watch-zone feature: the domain model, the Cosmos
// store over the WatchZones container, and the /v1/me/watch-zones HTTP handlers.
// It mirrors the .NET TownCrier.{Domain,Application,Infrastructure}.WatchZones
// slices (GH#418 iteration 5) but follows idiomatic Go — a plain struct
// validated at construction, a consumer-side store interface, and hand-written
// test fakes.
//
// Scope note: POST create (whose response body carries nearby applications) and
// GET /{zoneId}/applications are deferred to bead tc-5847 — they hard-depend on
// the geo/application stores that land in later iterations. This package ships
// list, update (PATCH), and delete; per-zone notification preferences live on
// the user profile and are served by the profiles package.
package watchzones

import (
	"errors"
	"strings"
	"time"
)

// WatchZone is a user's geofenced monitoring area: a circle (centre + radius)
// scoped to one planning authority. Exported fields keep it a plain Go value;
// the constructor enforces the invariants .NET guards in WatchZone's ctor.
type WatchZone struct {
	ID                  string
	UserID              string
	Name                string
	Latitude            float64
	Longitude           float64
	RadiusMetres        float64
	AuthorityID         int
	CreatedAt           time.Time
	PushEnabled         bool
	EmailInstantEnabled bool
}

// NewWatchZone validates and constructs a watch zone. It mirrors the .NET
// WatchZone constructor: id, user id and name must be non-blank and radius and
// authority id must be positive. Coordinate range is deliberately NOT checked
// here — like .NET, that is an HTTP-layer validation, so the domain accepts any
// latitude/longitude.
func NewWatchZone(id, userID, name string, latitude, longitude, radiusMetres float64, authorityID int, createdAt time.Time, pushEnabled, emailInstantEnabled bool) (WatchZone, error) {
	if strings.TrimSpace(id) == "" {
		return WatchZone{}, errors.New("id is required")
	}
	if strings.TrimSpace(userID) == "" {
		return WatchZone{}, errors.New("user id is required")
	}
	if strings.TrimSpace(name) == "" {
		return WatchZone{}, errors.New("name is required")
	}
	if radiusMetres <= 0 {
		return WatchZone{}, errors.New("radius must be positive")
	}
	if authorityID <= 0 {
		return WatchZone{}, errors.New("authority id must be positive")
	}
	return WatchZone{
		ID:                  id,
		UserID:              userID,
		Name:                name,
		Latitude:            latitude,
		Longitude:           longitude,
		RadiusMetres:        radiusMetres,
		AuthorityID:         authorityID,
		CreatedAt:           createdAt,
		PushEnabled:         pushEnabled,
		EmailInstantEnabled: emailInstantEnabled,
	}, nil
}

// ZoneUpdate is a partial PATCH: a nil field leaves the existing value
// untouched, mirroring the nullable parameters of .NET's WatchZone.WithUpdates.
type ZoneUpdate struct {
	Name                *string
	Latitude            *float64
	Longitude           *float64
	RadiusMetres        *float64
	AuthorityID         *int
	PushEnabled         *bool
	EmailInstantEnabled *bool
}

// WithUpdates returns a copy of the zone with the non-nil fields of u applied,
// re-validated through the constructor — so a merge that would violate an
// invariant (e.g. a blank name) returns an error rather than a corrupt zone,
// exactly as .NET's WithUpdates re-runs its guards. Identity (id, user id) and
// the creation timestamp are immutable across an update.
func (z WatchZone) WithUpdates(u ZoneUpdate) (WatchZone, error) {
	updated := z
	if u.Name != nil {
		updated.Name = *u.Name
	}
	if u.Latitude != nil {
		updated.Latitude = *u.Latitude
	}
	if u.Longitude != nil {
		updated.Longitude = *u.Longitude
	}
	if u.RadiusMetres != nil {
		updated.RadiusMetres = *u.RadiusMetres
	}
	if u.AuthorityID != nil {
		updated.AuthorityID = *u.AuthorityID
	}
	if u.PushEnabled != nil {
		updated.PushEnabled = *u.PushEnabled
	}
	if u.EmailInstantEnabled != nil {
		updated.EmailInstantEnabled = *u.EmailInstantEnabled
	}
	return NewWatchZone(
		updated.ID,
		updated.UserID,
		updated.Name,
		updated.Latitude,
		updated.Longitude,
		updated.RadiusMetres,
		updated.AuthorityID,
		updated.CreatedAt,
		updated.PushEnabled,
		updated.EmailInstantEnabled,
	)
}
