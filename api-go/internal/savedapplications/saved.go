// Package savedapplications owns the saved-application feature: the domain
// record, its Postgres store, snapshot refresh, and the
// /v1/me/saved-applications HTTP handlers.
package savedapplications

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// SavedApplication is a user's bookmark of a planning application. Identity is
// (UserID, ApplicationUID, AuthorityID): PlanIt uids are only unique within a
// council. The embedded Application snapshot is nil only for legacy rows
// persisted before the snapshot column existed.
type SavedApplication struct {
	UserID         string
	ApplicationUID string
	AuthorityID    int
	SavedAt        time.Time
	Application    *applications.PlanningApplication
}

// NewSavedApplication builds a saved record from a planning application, keyed on
// the canonical {areaId}/{name} uid (not the raw client-supplied uid) so re-saves
// of the same application are idempotent. The snapshot is embedded.
func NewSavedApplication(userID string, app applications.PlanningApplication, now time.Time) SavedApplication {
	return SavedApplication{
		UserID:         userID,
		ApplicationUID: app.CanonicalUID(),
		AuthorityID:    app.AreaID,
		SavedAt:        now,
		Application:    &app,
	}
}

// withEmbeddedSnapshot returns a copy of the saved record with app embedded as
// its snapshot, keeping the existing ApplicationUID and SavedAt.
func (s SavedApplication) withEmbeddedSnapshot(app applications.PlanningApplication) SavedApplication {
	s.Application = &app
	return s
}
