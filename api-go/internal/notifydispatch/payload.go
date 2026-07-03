package notifydispatch

import (
	"encoding/json"
	"fmt"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/vocabulary"
)

// savedThreadID is the aps.thread-id value for a zone-less (saved) push — both
// a single saved alert and the saved summary bucket — so Notification Center
// groups a user's saved-application pushes together regardless of any watch
// zone that also matched (GH#784).
const savedThreadID = "saved"

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
	Alert    apnsAlertContent `json:"alert"`
	Sound    string           `json:"sound"`
	Badge    int              `json:"badge"`
	ThreadID string           `json:"thread-id"`
}

type apnsAlertContent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// apnsSummaryPayload is the APNs body for a coalesced push covering more than
// one queued notification in a bucket (a watch zone, or the per-user saved
// bucket): a digest-style {aps:{alert,sound,badge,thread-id}} shape plus a
// routing "kind" discriminator and, for a zone bucket, the watchZoneId. It
// deliberately carries no applicationRef/notificationId — that absence is how
// iOS knows to open the in-app notifications list rather than a single
// application (GH#784).
type apnsSummaryPayload struct {
	Aps         apnsSummaryAps `json:"aps"`
	Kind        string         `json:"kind"`
	WatchZoneID *string        `json:"watchZoneId,omitempty"`
}

type apnsSummaryAps struct {
	Alert    apnsAlertContent `json:"alert"`
	Sound    string           `json:"sound"`
	Badge    int              `json:"badge"`
	ThreadID string           `json:"thread-id"`
}

// buildAlertPayload renders the instant-push body for a notification: a
// zone-matched notification is titled "Planning update near you", a saved-only
// one "Town Crier"; a decision update body appends the UK display label,
// otherwise the body is the address. totalUnreadCount is the app-icon badge.
// aps.thread-id is the watch zone id for a zone notification, "saved" for a
// zone-less one, so iOS groups a zone's pushes together in Notification Center.
func buildAlertPayload(n notifications.DigestNotification, totalUnreadCount int) (json.RawMessage, error) {
	title := "Town Crier"
	threadID := savedThreadID
	if n.WatchZoneID != nil {
		title = "Planning update near you"
		threadID = *n.WatchZoneID
	}
	body := n.ApplicationAddress
	if n.EventType == notifications.EventDecisionUpdate {
		body = buildDecisionBody(n)
	}

	raw, err := json.Marshal(apnsAlertPayload{
		Aps: apnsAlertAps{
			Alert:    apnsAlertContent{Title: title, Body: body},
			Sound:    "default",
			Badge:    totalUnreadCount,
			ThreadID: threadID,
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

// buildZoneSummaryPayload renders the coalesced push for a watch zone bucket
// that queued more than one notification this cycle: "Planning updates near
// {zoneName}" / "{count} updates in this area. Tap to see them." threadID is
// the watch zone id (also surfaced as watchZoneId, the iOS routing hint).
func buildZoneSummaryPayload(count int, zoneName string, badge int, threadID string) (json.RawMessage, error) {
	title := fmt.Sprintf("Planning updates near %s", zoneName)
	body := fmt.Sprintf("%d updates in this area. Tap to see them.", count)

	raw, err := json.Marshal(apnsSummaryPayload{
		Aps: apnsSummaryAps{
			Alert:    apnsAlertContent{Title: title, Body: body},
			Sound:    "default",
			Badge:    badge,
			ThreadID: threadID,
		},
		Kind:        "zoneSummary",
		WatchZoneID: &threadID,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal zone summary payload: %w", err)
	}
	return raw, nil
}

// buildSavedSummaryPayload renders the coalesced push for the per-user saved
// (zone-less) bucket when it queued more than one notification this cycle:
// "Your saved applications" / "{count} have a decision. Tap to see them."
func buildSavedSummaryPayload(count, badge int) (json.RawMessage, error) {
	body := fmt.Sprintf("%d have a decision. Tap to see them.", count)

	raw, err := json.Marshal(apnsSummaryPayload{
		Aps: apnsSummaryAps{
			Alert:    apnsAlertContent{Title: "Your saved applications", Body: body},
			Sound:    "default",
			Badge:    badge,
			ThreadID: savedThreadID,
		},
		Kind: "savedSummary",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal saved summary payload: %w", err)
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
