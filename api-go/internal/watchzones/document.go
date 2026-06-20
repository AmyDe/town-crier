package watchzones

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// watchZoneDocument is the Cosmos persistence shape for a WatchZone. The JSON
// tags use camelCase so a document written here is byte-compatible with the
// existing WatchZones container and an existing document hydrates unchanged.
//
// Partition key: the WatchZones container is partitioned by /userId; the
// document id equals the zone id. Every single-zone operation is therefore a
// point operation keyed on (userId, zoneId), and a user's zones are listed with
// a single-partition query on userId.
type watchZoneDocument struct {
	ID           string  `json:"id"`
	UserID       string  `json:"userId"`
	Name         string  `json:"name"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	RadiusMetres float64 `json:"radiusMetres"`
	AuthorityID  int     `json:"authorityId"`

	// createdAt must serialise with a numeric UTC offset ("+00:00"), never Go's
	// RFC 3339 "Z" — platform.DotNetTime handles that. Nullable so a legacy
	// document missing it hydrates to the zero instant (time.Time{}).
	CreatedAt *platform.DotNetTime `json:"createdAt"`

	// pushEnabled / emailInstantEnabled are pointers so a document predating the
	// per-zone flags (tc-kh1s) hydrates as opt-in (true) rather than the Go zero
	// value (false) — absent bools coalesce to true.
	PushEnabled         *bool `json:"pushEnabled"`
	EmailInstantEnabled *bool `json:"emailInstantEnabled"`
}

// newWatchZoneDocument maps a domain zone to its persistence shape. The flags are
// always written explicitly (never absent) so a freshly written document carries
// the user's actual preference; createdAt is written with a numeric UTC offset.
func newWatchZoneDocument(z WatchZone) watchZoneDocument {
	createdAt := platform.DotNetTime(z.CreatedAt)
	push := z.PushEnabled
	email := z.EmailInstantEnabled
	return watchZoneDocument{
		ID:                  z.ID,
		UserID:              z.UserID,
		Name:                z.Name,
		Latitude:            z.Latitude,
		Longitude:           z.Longitude,
		RadiusMetres:        z.RadiusMetres,
		AuthorityID:         z.AuthorityID,
		CreatedAt:           &createdAt,
		PushEnabled:         &push,
		EmailInstantEnabled: &email,
	}
}

// toDomain reconstitutes a domain zone from its stored document, coalescing the
// legacy-nullable flags to true and an absent createdAt to the zero instant.
func (d watchZoneDocument) toDomain() (WatchZone, error) {
	var createdAt time.Time
	if d.CreatedAt != nil {
		createdAt = time.Time(*d.CreatedAt)
	}
	return NewWatchZone(
		d.ID,
		d.UserID,
		d.Name,
		d.Latitude,
		d.Longitude,
		d.RadiusMetres,
		d.AuthorityID,
		createdAt,
		coalesceTrue(d.PushEnabled),
		coalesceTrue(d.EmailInstantEnabled),
	)
}

// coalesceTrue defaults an absent (nil) nullable flag to true, preserving the
// opt-in default for documents written before the flag existed.
func coalesceTrue(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}
