// Package savedapplications owns the saved-application feature: the domain
// record, the Cosmos store over the SavedApplications container, and the
// /v1/me/saved-applications HTTP handlers (GH#418 iteration 6).
//
// Scope note: this ships the fresh-data path (a save always embeds the
// snapshot and keys on the canonical uid). The legacy-row migration machinery
// — lazy snapshot backfill, legacy-uid re-keying and dedup, plus refresh-on-tap
// — is deferred to bead tc-wans.
package savedapplications

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// SavedApplication is a user's bookmark of a planning application. Identity is
// (UserID, ApplicationUID, AuthorityID): PlanIt uids are only unique within a
// council. The embedded Application snapshot lets the list endpoint render with
// one partitioned query rather than an N-fan-out hydration; it is nil only for
// legacy rows persisted before the snapshot column existed.
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
	snapshot := app
	return SavedApplication{
		UserID:         userID,
		ApplicationUID: app.CanonicalUID(),
		AuthorityID:    app.AreaID,
		SavedAt:        now,
		Application:    &snapshot,
	}
}

// withEmbeddedSnapshot returns a copy of the saved record with app embedded as
// its snapshot, keeping the existing ApplicationUID and SavedAt. Re-keying to the
// canonical uid is a separate step, so the lazy backfill and the re-key stay
// independently testable.
func (s SavedApplication) withEmbeddedSnapshot(app applications.PlanningApplication) SavedApplication {
	snapshot := app
	s.Application = &snapshot
	return s
}
