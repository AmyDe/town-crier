package notifydispatch

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
)

// NotificationSources values are the flag names used for the stored sources
// string (comma-joined when both apply), preserved verbatim because the field
// is read by the digest worker's HasSavedSource check and the web API.
const (
	sourceZone  = "Zone"
	sourceSaved = "Saved"
)

// recordInput collects the fields needed to mint a notification record, so the
// enqueuer and decision dispatcher build the digest-readable record the same way.
type recordInput struct {
	id          string
	userID      string
	app         applications.PlanningApplication
	watchZoneID *string
	eventType   notifications.EventType
	sources     string
	now         time.Time
}

// newRecord builds the notification record from a polled application. The result
// is a notifications.DigestNotification — the exact shape the digest worker reads
// (ByUserSince / UnsentEmailsByUser) and writes — so a record created here flows
// into the weekly and hourly digests unchanged. A decision update carries the
// PlanIt app_state string; a new application leaves Decision nil.
func newRecord(in recordInput) notifications.DigestNotification {
	n := notifications.DigestNotification{
		ID:                     in.id,
		UserID:                 in.userID,
		ApplicationUID:         in.app.UID,
		ApplicationName:        in.app.Name,
		WatchZoneID:            in.watchZoneID,
		ApplicationAddress:     in.app.Address,
		ApplicationDescription: in.app.Description,
		ApplicationType:        in.app.AppType,
		AuthorityID:            in.app.AreaID,
		EventType:              in.eventType,
		Sources:                in.sources,
		CreatedAt:              in.now,
	}
	if in.eventType == notifications.EventDecisionUpdate {
		n.Decision = in.app.AppState
	}
	return n
}
