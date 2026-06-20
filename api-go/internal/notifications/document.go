package notifications

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// notificationDocument is the read projection of a Notifications-container
// document. Only the fields the latest-unread lookup needs are declared; the
// SELECT * query returns the full document, and json.Unmarshal ignores the rest.
// JSON keys are camelCase. eventType is a pointer so legacy rows predating the
// field (stored null) coalesce to NewApplication on read.
type notificationDocument struct {
	ApplicationUID *string             `json:"applicationUid"`
	Decision       *string             `json:"decision"`
	EventType      *string             `json:"eventType"`
	CreatedAt      platform.DotNetTime `json:"createdAt"`
}

// toLatestUnread hydrates the descriptor, coalescing a null/empty eventType to
// NewApplication and a null applicationUid to the empty string (legacy rows).
func (d notificationDocument) toLatestUnread() LatestUnread {
	eventType := EventNewApplication
	if d.EventType != nil && *d.EventType != "" {
		eventType = EventType(*d.EventType)
	}
	uid := ""
	if d.ApplicationUID != nil {
		uid = *d.ApplicationUID
	}
	return LatestUnread{
		ApplicationUID: uid,
		EventType:      eventType,
		Decision:       d.Decision,
		CreatedAt:      time.Time(d.CreatedAt),
	}
}
