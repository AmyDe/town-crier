package notifications

import (
	"strings"
	"time"
)

// DigestNotification is a complete stored notification record.
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
	// PushSent means "a push was queued", never "delivered": it is set before
	// the coalescer sends and is deliberately never reconciled against the
	// delivery outcome. Only the user's data export reads it.
	PushSent  bool
	EmailSent bool
	CreatedAt time.Time
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
