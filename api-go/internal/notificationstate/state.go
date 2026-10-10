// Package notificationstate owns the per-user notification read state: the
// version change token and the mark-read operations over notifications.read_at
// (ADR 0035). A notification is unread iff read_at IS NULL.
package notificationstate

import "time"

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
