// Package notifications provides read access to the Notifications container for
// the per-application latest-unread lookup that augments the
// applications-by-zone endpoint (GH#418). Full notification dispatch and
// digest generation live in the worker, out of scope for this package.
package notifications

import "time"

// EventType is the lifecycle event a notification was raised for. The string
// forms ("NewApplication", "DecisionUpdate") are the exact values stored in
// Cosmos and emitted on the wire, preserved verbatim here.
type EventType string

const (
	// EventNewApplication marks a notification raised because an application
	// first appeared and matched a subscription.
	EventNewApplication EventType = "NewApplication"
	// EventDecisionUpdate marks a notification raised because a tracked
	// application transitioned into a decision state.
	EventDecisionUpdate EventType = "DecisionUpdate"
)

// NotificationCounts is a per-user notification tally: the total number of
// notifications and how many are still unread (read_at IS NULL). It backs the
// admin user list's "unread/total" column and is deliberately kept off
// UserProfile — it is a list-view aggregate, not a profile property.
type NotificationCounts struct {
	Total  int
	Unread int
}

// NotificationTotals is the global notification tally across all users: the
// number sent and how many are still unread (read_at IS NULL). It backs the
// admin stats "reach" block — a whole-table aggregate, distinct from the
// per-user NotificationCounts used on the user list.
type NotificationTotals struct {
	Sent   int
	Unread int
}

// LatestUnread is the per-application unread descriptor surfaced on each row of
// the applications-by-zone result: the event, the optional PlanIt decision
// string (decision updates only), and when the notification was raised.
type LatestUnread struct {
	ApplicationUID string
	EventType      EventType
	Decision       *string
	CreatedAt      time.Time
}
