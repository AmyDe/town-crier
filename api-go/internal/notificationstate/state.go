// Package notificationstate owns the per-user notification read-state feature:
// the State value (a version change token plus a vestigial last-read instant),
// the mark-read / mark-all-read read-state mutations over the notifications
// table, the unread count, and the GET/mark-all-read/mark-read HTTP handlers.
//
// Read state is per-application, keyed on notifications.read_at (ADR 0035,
// superseding the last-read watermark). A notification is unread iff
// read_at IS NULL; opening an application clears its rows (mark-read); mark-all
// clears every unread row. The notification_state.version is retained as an
// opaque change token that bumps on any read-state mutation so the client's
// BadgeSync still detects change; last_read_at no longer drives unread.
package notificationstate

import (
	"errors"
	"strings"
	"time"
)

// State is one user's notification read-state row. Version is the opaque change
// token that increments on every read-state mutation (mark-read that cleared at
// least one row, and mark-all-read). LastReadAt is retained for GET DTO shape
// stability only; it no longer drives the unread computation (that is
// read_at IS NULL on the notifications table — ADR 0035).
type State struct {
	UserID     string
	LastReadAt time.Time
	Version    int
}

// NewState seeds a first-touch state row at the given instant with version 1.
// LastReadAt is vestigial (kept for DTO shape); the meaningful field is Version,
// the change token BadgeSync observes.
func NewState(userID string, lastReadAt time.Time) (State, error) {
	if strings.TrimSpace(userID) == "" {
		return State{}, errors.New("user id is required")
	}
	return State{UserID: userID, LastReadAt: lastReadAt, Version: 1}, nil
}

// MarkAllReadAt stamps LastReadAt to now and bumps the version change token.
func (s *State) MarkAllReadAt(now time.Time) {
	s.LastReadAt = now
	s.Version++
}
