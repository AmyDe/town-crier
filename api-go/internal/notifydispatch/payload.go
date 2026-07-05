package notifydispatch

import (
	"encoding/json"
	"fmt"
	"strconv"

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
	title, body := alertTitleBody(n)
	threadID := savedThreadID
	if n.WatchZoneID != nil {
		threadID = *n.WatchZoneID
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
	title, body := zoneSummaryTitleBody(count, zoneName)

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
	title, body := savedSummaryTitleBody(count)

	raw, err := json.Marshal(apnsSummaryPayload{
		Aps: apnsSummaryAps{
			Alert:    apnsAlertContent{Title: title, Body: body},
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

// alertTitleBody, zoneSummaryTitleBody, and savedSummaryTitleBody render the
// server-side title/body copy each push shape carries. They are shared by the
// APNs and FCM builders so the two platforms always render byte-identical
// strings for the same event — the copy can never drift between them.

// alertTitleBody renders the instant-push title/body: a zone-matched
// notification is titled "Planning update near you", a saved-only one "Town
// Crier"; a decision update body appends the UK display label, otherwise the
// body is the address.
func alertTitleBody(n notifications.DigestNotification) (title, body string) {
	title = "Town Crier"
	if n.WatchZoneID != nil {
		title = "Planning update near you"
	}
	body = n.ApplicationAddress
	if n.EventType == notifications.EventDecisionUpdate {
		body = buildDecisionBody(n)
	}
	return title, body
}

// zoneSummaryTitleBody renders the coalesced watch-zone summary copy.
func zoneSummaryTitleBody(count int, zoneName string) (title, body string) {
	return fmt.Sprintf("Planning updates near %s", zoneName),
		fmt.Sprintf("%d updates in this area. Tap to see them.", count)
}

// savedSummaryTitleBody renders the coalesced saved-bucket summary copy.
func savedSummaryTitleBody(count int) (title, body string) {
	return "Your saved applications",
		fmt.Sprintf("%d have a decision. Tap to see them.", count)
}

// fcmMessage is the FCM HTTP v1 "message" object minus the per-recipient token
// (fcm.Client injects the token itself). It mirrors the three APNs shapes: a
// system-rendered notification (title/body, same strings as APNs) plus a data
// dictionary carrying the routing "kind" discriminator and — for the single
// alert — the deep-link keys the Android client's tap-routing reads, mirroring
// iOS's custom-key parsing. FCM data values must all be strings, so authorityId
// is stringified. There is no badge field (Android badges are channel-driven).
type fcmMessage struct {
	Notification fcmNotification   `json:"notification"`
	Android      fcmAndroidConfig  `json:"android"`
	Data         map[string]string `json:"data"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type fcmAndroidConfig struct {
	Priority     string                 `json:"priority"`
	Notification fcmAndroidNotification `json:"notification"`
}

type fcmAndroidNotification struct {
	ChannelID string `json:"channel_id"`
}

// fcmAlertChannelID is the Android notification channel the pushes post to. The
// #777 client registers this channel; day-1 all shapes share it.
const fcmAlertChannelID = "alerts"

// marshalFCMMessage builds the token-less FCM message body from a rendered
// title/body and the data dictionary.
func marshalFCMMessage(title, body string, data map[string]string) (json.RawMessage, error) {
	raw, err := json.Marshal(fcmMessage{
		Notification: fcmNotification{Title: title, Body: body},
		Android: fcmAndroidConfig{
			Priority:     "HIGH",
			Notification: fcmAndroidNotification{ChannelID: fcmAlertChannelID},
		},
		Data: data,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal fcm message: %w", err)
	}
	return raw, nil
}

// buildBucketFCMPayload renders one bucket's FCM message body, mirroring
// buildBucketPayload: the rich single-app alert (with deep-link data) when
// exactly one notification queued, otherwise the zone or saved summary.
func buildBucketFCMPayload(bucketKey string, queued []notifications.DigestNotification, zoneNames map[string]string) (json.RawMessage, error) {
	if len(queued) == 1 {
		return buildAlertFCMPayload(queued[0])
	}
	if bucketKey == savedBucket {
		return buildSavedSummaryFCMPayload(len(queued))
	}
	return buildZoneSummaryFCMPayload(len(queued), zoneNames[bucketKey], bucketKey)
}

// buildAlertFCMPayload renders the instant-push FCM body for a single
// notification, carrying the same deep-link keys as the APNs alert
// (notificationId / applicationRef / authorityId / createdAt) so the Android
// client's tap-routing mirrors iOS. authorityId is stringified (FCM data values
// are strings); there is no badge (Android badges are channel-driven).
func buildAlertFCMPayload(n notifications.DigestNotification) (json.RawMessage, error) {
	title, body := alertTitleBody(n)
	return marshalFCMMessage(title, body, map[string]string{
		"kind":           "alert",
		"notificationId": n.ID,
		"applicationRef": n.ApplicationName,
		"authorityId":    strconv.Itoa(n.AuthorityID),
		"createdAt":      platform.DotNetTime(n.CreatedAt).String(),
	})
}

// buildZoneSummaryFCMPayload renders the coalesced watch-zone summary FCM body.
// It carries kind=zoneSummary and watchZoneId (the #777 routing discriminators)
// and, deliberately, none of the single-app deep-link keys.
func buildZoneSummaryFCMPayload(count int, zoneName, watchZoneID string) (json.RawMessage, error) {
	title, body := zoneSummaryTitleBody(count, zoneName)
	return marshalFCMMessage(title, body, map[string]string{
		"kind":        "zoneSummary",
		"watchZoneId": watchZoneID,
	})
}

// buildSavedSummaryFCMPayload renders the coalesced saved-bucket summary FCM
// body: kind=savedSummary, no watchZoneId, no deep-link keys.
func buildSavedSummaryFCMPayload(count int) (json.RawMessage, error) {
	title, body := savedSummaryTitleBody(count)
	return marshalFCMMessage(title, body, map[string]string{
		"kind": "savedSummary",
	})
}
