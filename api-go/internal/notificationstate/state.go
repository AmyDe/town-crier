// Package notificationstate owns the per-user read-watermark feature: the
// State domain value, its Cosmos document shape, the store (watermark document
// + unread count over the Notifications container), and the
// GET/mark-all-read/advance HTTP handlers (GH#418 iteration 4).
package notificationstate

import (
	"errors"
	"strings"
	"time"
)

// State is one user's notification read watermark. LastReadAt is the cutoff
// (notifications created at exactly that instant count as read); Version
// increments on every change.
type State struct {
	UserID     string
	LastReadAt time.Time
	Version    int
}

// NewState seeds a first-touch watermark at the given instant with version 1,
// matching NotificationStateAggregate.Create: all existing notifications count
// as already read ("clean slate", spec pre-resolved decision #13).
func NewState(userID string, lastReadAt time.Time) (State, error) {
	if strings.TrimSpace(userID) == "" {
		return State{}, errors.New("user id is required")
	}
	return State{UserID: userID, LastReadAt: lastReadAt, Version: 1}, nil
}

// MarkAllReadAt moves the watermark to now unconditionally and bumps the
// version, mirroring MarkAllReadAt.
func (s *State) MarkAllReadAt(now time.Time) {
	s.LastReadAt = now
	s.Version++
}

// AdvanceTo moves the watermark forward to asOf, reporting whether anything
// changed. A stale asOf (<= the current watermark) is a no-op returning false,
// mirroring AdvanceTo's redundant-write guard.
func (s *State) AdvanceTo(asOf time.Time) bool {
	if !asOf.After(s.LastReadAt) {
		return false
	}
	s.LastReadAt = asOf
	s.Version++
	return true
}
