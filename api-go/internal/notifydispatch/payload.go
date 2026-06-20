package notifydispatch

import (
	"encoding/json"
	"fmt"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/vocabulary"
)

// apnsAlertPayload is the APNs body for a single-notification (instant) push.
// JSON keys are camelCase; the top-level notificationId / applicationRef /
// authorityId / createdAt fields are the deep-link metadata iOS reads.
type apnsAlertPayload struct {
	Aps            apnsAlertAps        `json:"aps"`
	NotificationID string              `json:"notificationId"`
	ApplicationRef string              `json:"applicationRef"`
	AuthorityID    int                 `json:"authorityId"`
	CreatedAt      platform.DotNetTime `json:"createdAt"`
}

type apnsAlertAps struct {
	Alert apnsAlertContent `json:"alert"`
	Sound string           `json:"sound"`
	Badge int              `json:"badge"`
}

type apnsAlertContent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// buildAlertPayload renders the instant-push body for a notification: a
// zone-matched notification is titled "Planning update near you", a saved-only
// one "Town Crier"; a decision update body appends the UK display label,
// otherwise the body is the address. totalUnreadCount is the app-icon badge.
func buildAlertPayload(n notifications.DigestNotification, totalUnreadCount int) (json.RawMessage, error) {
	title := "Town Crier"
	if n.WatchZoneID != nil {
		title = "Planning update near you"
	}
	body := n.ApplicationAddress
	if n.EventType == notifications.EventDecisionUpdate {
		body = buildDecisionBody(n)
	}

	raw, err := json.Marshal(apnsAlertPayload{
		Aps: apnsAlertAps{
			Alert: apnsAlertContent{Title: title, Body: body},
			Sound: "default",
			Badge: totalUnreadCount,
		},
		NotificationID: n.ID,
		ApplicationRef: n.ApplicationName,
		AuthorityID:    n.AuthorityID,
		CreatedAt:      platform.DotNetTime(n.CreatedAt),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal alert payload: %w", err)
	}
	return raw, nil
}

// buildDecisionBody appends the UK display label to the address for a decision
// update ("10 High St — Approved"), falling back to the bare address when the
// decision string is empty or unrecognised.
func buildDecisionBody(n notifications.DigestNotification) string {
	label := vocabulary.UKDisplayString(n.Decision)
	if label == "" {
		return n.ApplicationAddress
	}
	return n.ApplicationAddress + " — " + label
}
