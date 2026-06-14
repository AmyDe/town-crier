package notifydispatch

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// apnsAlertPayload is the APNs body for a single-notification (instant) push.
// The JSON keys reproduce the .NET ApnsAlertPayload / ApnsAlertAps /
// ApnsAlertContent shapes exactly so the iOS client receives an unchanged
// payload across the cutover: the top-level notificationId / applicationRef /
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

// buildAlertPayload renders the instant-push body for a notification, mirroring
// .NET ApnsPushNotificationSender.BuildAlertPayload: a zone-matched notification
// is titled "Planning update near you", a saved-only one "Town Crier"; a decision
// update body appends the UK display label, otherwise the body is the address.
// totalUnreadCount is the app-icon badge.
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
// decision string is empty or unrecognised. Mirrors .NET BuildDecisionBody.
func buildDecisionBody(n notifications.DigestNotification) string {
	label := ukDisplayString(n.Decision)
	if label == "" {
		return n.ApplicationAddress
	}
	return n.ApplicationAddress + " — " + label
}

// ukDisplayString maps a raw PlanIt app_state to the UK planning term residents
// recognise, returning "" for a nil or unrecognised input. Port of .NET
// UkPlanningVocabulary.GetDisplayString (matches the digest worker's copy).
func ukDisplayString(planItAppState *string) string {
	if planItAppState == nil {
		return ""
	}
	state := strings.TrimSpace(*planItAppState)
	switch {
	case strings.EqualFold(state, "Permitted"):
		return "Approved"
	case strings.EqualFold(state, "Conditions"):
		return "Approved with conditions"
	case strings.EqualFold(state, "Rejected"):
		return "Refused"
	case strings.EqualFold(state, "Appealed"):
		return "Refusal appealed"
	default:
		return ""
	}
}
