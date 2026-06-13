package savedapplications

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// savedApplicationDocument is the Cosmos persistence shape for a SavedApplication
// in the SavedApplications container. JSON tags reproduce the camelCase keys the
// .NET CosmosSavedApplicationRepository writes.
//
// Partition key: userId. Document id: "{userId}:{applicationUid}" — so a save is
// an idempotent point upsert and a user's list is one single-partition query.
type savedApplicationDocument struct {
	ID             string                         `json:"id"`
	UserID         string                         `json:"userId"`
	ApplicationUID string                         `json:"applicationUid"`
	AuthorityID    *int                           `json:"authorityId"`
	SavedAt        platform.DotNetTime            `json:"savedAt"`
	Application    *applications.SnapshotDocument `json:"application"`
}

// makeID builds the composite document id, matching .NET SavedApplicationDocument.MakeId.
func makeID(userID, applicationUID string) string {
	return userID + ":" + applicationUID
}

// newSavedApplicationDocument maps a domain record to its persistence shape.
func newSavedApplicationDocument(s SavedApplication) savedApplicationDocument {
	var snapshot *applications.SnapshotDocument
	if s.Application != nil {
		d := applications.NewSnapshotDocument(*s.Application)
		snapshot = &d
	}
	authorityID := s.AuthorityID
	return savedApplicationDocument{
		ID:             makeID(s.UserID, s.ApplicationUID),
		UserID:         s.UserID,
		ApplicationUID: s.ApplicationUID,
		AuthorityID:    &authorityID,
		SavedAt:        platform.DotNetTime(s.SavedAt),
		Application:    snapshot,
	}
}

// toDomain reconstitutes a domain record, coalescing the projected authorityId
// from the embedded snapshot for legacy rows persisted before that column
// existed, exactly as .NET does.
func (d savedApplicationDocument) toDomain() SavedApplication {
	authorityID := 0
	switch {
	case d.AuthorityID != nil:
		authorityID = *d.AuthorityID
	case d.Application != nil:
		authorityID = d.Application.AreaID
	}

	var app *applications.PlanningApplication
	if d.Application != nil {
		a := d.Application.ToDomain()
		app = &a
	}

	return SavedApplication{
		UserID:         d.UserID,
		ApplicationUID: d.ApplicationUID,
		AuthorityID:    authorityID,
		SavedAt:        time.Time(d.SavedAt),
		Application:    app,
	}
}
