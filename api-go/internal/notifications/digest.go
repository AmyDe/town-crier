package notifications

import (
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// ninetyDaysSeconds is the TTL applied to notification documents so dispatched
// notifications expire after 90 days.
const ninetyDaysSeconds = 90 * 24 * 60 * 60

// DigestNotification is the full read/write model the digest worker needs — the
// complete Notifications-container document, not the latest-unread projection
// used by the web API. It carries every field the digest email body and APNs
// payload render, plus the EmailSent flag the hourly cycle flips after a
// successful send.
type DigestNotification struct {
	ID                     string
	UserID                 string
	ApplicationUID         string
	ApplicationName        string
	WatchZoneID            *string
	ApplicationAddress     string
	ApplicationDescription string
	ApplicationType        *string
	AuthorityID            int
	Decision               *string
	EventType              EventType
	Sources                string
	PushSent               bool
	EmailSent              bool
	CreatedAt              time.Time
}

// HasSavedSource reports whether the notification was produced (in part) by the
// user's Saved list, used to render the "★ saved" indicator on a zone card.
func (n DigestNotification) HasSavedSource() bool {
	for _, part := range strings.Split(n.Sources, ",") {
		if strings.EqualFold(strings.TrimSpace(part), "Saved") {
			return true
		}
	}
	return false
}

// MarkEmailSent flips EmailSent so the persisted document is excluded from the
// next hourly cycle's unsent-emails query.
func (n *DigestNotification) MarkEmailSent() {
	n.EmailSent = true
}

// digestDocument is the full Cosmos persistence shape of a Notifications-container
// document. JSON keys are camelCase so Go-written documents are byte-compatible
// with the existing container. Nullable pointer fields hydrate legacy rows
// (predating eventType/sources/applicationUid) with the backfill defaults.
type digestDocument struct {
	ID                     string              `json:"id"`
	UserID                 string              `json:"userId"`
	ApplicationUID         *string             `json:"applicationUid"`
	ApplicationName        string              `json:"applicationName"`
	WatchZoneID            *string             `json:"watchZoneId"`
	ApplicationAddress     string              `json:"applicationAddress"`
	ApplicationDescription string              `json:"applicationDescription"`
	ApplicationType        *string             `json:"applicationType"`
	AuthorityID            int                 `json:"authorityId"`
	Decision               *string             `json:"decision"`
	EventType              *string             `json:"eventType"`
	Sources                *string             `json:"sources"`
	PushSent               bool                `json:"pushSent"`
	EmailSent              bool                `json:"emailSent"`
	CreatedAt              platform.DotNetTime `json:"createdAt"`
	TTL                    int                 `json:"ttl"`
}

// toDigest hydrates the full digest model from a stored document, coalescing the
// legacy nulls: a null eventType becomes NewApplication, a null applicationUid
// falls back to applicationName, and an absent sources stays empty (only the
// Saved check reads it, and a legacy Zone-only row has no Saved indicator).
func (d digestDocument) toDigest() DigestNotification {
	eventType := EventNewApplication
	if d.EventType != nil && *d.EventType != "" {
		eventType = EventType(*d.EventType)
	}
	uid := d.ApplicationName
	if d.ApplicationUID != nil && *d.ApplicationUID != "" {
		uid = *d.ApplicationUID
	}
	sources := ""
	if d.Sources != nil {
		sources = *d.Sources
	}
	return DigestNotification{
		ID:                     d.ID,
		UserID:                 d.UserID,
		ApplicationUID:         uid,
		ApplicationName:        d.ApplicationName,
		WatchZoneID:            d.WatchZoneID,
		ApplicationAddress:     d.ApplicationAddress,
		ApplicationDescription: d.ApplicationDescription,
		ApplicationType:        d.ApplicationType,
		AuthorityID:            d.AuthorityID,
		Decision:               d.Decision,
		EventType:              eventType,
		Sources:                sources,
		PushSent:               d.PushSent,
		EmailSent:              d.EmailSent,
		CreatedAt:              time.Time(d.CreatedAt),
	}
}

// newDigestDocument maps the digest model back to its persistence shape for the
// MarkEmailSent upsert. It always writes the 90-day TTL so re-saved documents
// keep the 90-day retention.
func newDigestDocument(n DigestNotification) digestDocument {
	eventType := string(n.EventType)
	sources := n.Sources
	uid := n.ApplicationUID
	return digestDocument{
		ID:                     n.ID,
		UserID:                 n.UserID,
		ApplicationUID:         &uid,
		ApplicationName:        n.ApplicationName,
		WatchZoneID:            n.WatchZoneID,
		ApplicationAddress:     n.ApplicationAddress,
		ApplicationDescription: n.ApplicationDescription,
		ApplicationType:        n.ApplicationType,
		AuthorityID:            n.AuthorityID,
		Decision:               n.Decision,
		EventType:              &eventType,
		Sources:                &sources,
		PushSent:               n.PushSent,
		EmailSent:              n.EmailSent,
		CreatedAt:              platform.DotNetTime(n.CreatedAt),
		TTL:                    ninetyDaysSeconds,
	}
}
