package notifydispatch

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/vocabulary"
)

// savedThreadID is the aps.thread-id for every zone-less (saved) push, so
// Notification Center groups saved pushes even when a watch zone also matched.
const savedThreadID = "saved"

// apnsAlertPayload is the APNs body for a single-notification (instant) push.
// The top-level fields are the deep-link metadata iOS reads.
type apnsAlertPayload struct {
	Aps            apnsAps             `json:"aps"`
	NotificationID string              `json:"notificationId"`
	ApplicationRef string              `json:"applicationRef"`
	AuthorityID    int                 `json:"authorityId"`
	CreatedAt      platform.DotNetTime `json:"createdAt"`
}

type apnsAps struct {
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
// one queued notification in a bucket. It deliberately carries no
// applicationRef/notificationId: that absence is how iOS knows to open the
// in-app notifications list rather than a single application.
type apnsSummaryPayload struct {
	Aps         apnsAps `json:"aps"`
	Kind        string  `json:"kind"`
	WatchZoneID *string `json:"watchZoneId,omitempty"`
}

// buildAlertPayload renders the instant-push body for a notification;
// totalUnreadCount is the app-icon badge.
func buildAlertPayload(n notifications.DigestNotification, totalUnreadCount int) (json.RawMessage, error) {
	title, body := alertTitleBody(n)
	threadID := savedThreadID
	if n.WatchZoneID != nil {
		threadID = *n.WatchZoneID
	}

	raw, err := json.Marshal(apnsAlertPayload{
		Aps: apnsAps{
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

// buildZoneSummaryPayload renders the coalesced push for a watch zone bucket;
// threadID is the watch zone id.
func buildZoneSummaryPayload(count int, zoneName string, badge int, threadID string) (json.RawMessage, error) {
	title, body := zoneSummaryTitleBody(count, zoneName)

	raw, err := json.Marshal(apnsSummaryPayload{
		Aps: apnsAps{
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
// (zone-less) bucket.
func buildSavedSummaryPayload(count, badge int) (json.RawMessage, error) {
	title, body := savedSummaryTitleBody(count)

	raw, err := json.Marshal(apnsSummaryPayload{
		Aps: apnsAps{
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

// alertTitleBody renders the instant-push title/body. A decision update with
// an empty or unrecognised decision falls back to the bare address.
func alertTitleBody(n notifications.DigestNotification) (title, body string) {
	title = "Town Crier"
	if n.WatchZoneID != nil {
		title = "Planning update near you"
	}
	body = n.ApplicationAddress
	if n.EventType == notifications.EventDecisionUpdate {
		if label := vocabulary.UKDisplayString(n.Decision); label != "" {
			body = n.ApplicationAddress + " — " + label
		}
	}
	return title, body
}

func zoneSummaryTitleBody(count int, zoneName string) (title, body string) {
	return fmt.Sprintf("Planning updates near %s", zoneName),
		fmt.Sprintf("%d updates in this area. Tap to see them.", count)
}

func savedSummaryTitleBody(count int) (title, body string) {
	return "Your saved applications",
		fmt.Sprintf("%d have a decision. Tap to see them.", count)
}

// fcmMessage is the FCM HTTP v1 "message" object minus the per-recipient token,
// which fcm.Client injects. FCM data values must all be strings, and there is no
// badge field because Android badges are channel-driven.
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
	Tag       string `json:"tag,omitempty"`
}

// fcmAlertChannelID is the Android notification channel all push shapes post
// to; the Android client registers it.
const fcmAlertChannelID = "alerts"

func marshalFCMMessage(title, body, tag string, data map[string]string) (json.RawMessage, error) {
	raw, err := json.Marshal(fcmMessage{
		Notification: fcmNotification{Title: title, Body: body},
		Android: fcmAndroidConfig{
			Priority:     "HIGH",
			Notification: fcmAndroidNotification{ChannelID: fcmAlertChannelID, Tag: tag},
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
func buildBucketFCMPayload(bucketKey string, queued []notifications.DigestNotification, zoneNames map[string]string, tag string) (json.RawMessage, error) {
	if len(queued) == 1 {
		return buildAlertFCMPayload(queued[0], tag)
	}
	if bucketKey == savedBucket {
		return buildSavedSummaryFCMPayload(len(queued), tag)
	}
	return buildZoneSummaryFCMPayload(len(queued), zoneNames[bucketKey], bucketKey, tag)
}

// buildAlertFCMPayload carries the same deep-link keys as the APNs alert so
// Android tap-routing mirrors iOS.
func buildAlertFCMPayload(n notifications.DigestNotification, tag string) (json.RawMessage, error) {
	title, body := alertTitleBody(n)
	return marshalFCMMessage(title, body, tag, map[string]string{
		"kind":           "alert",
		"notificationId": n.ID,
		"applicationRef": n.ApplicationName,
		"authorityId":    strconv.Itoa(n.AuthorityID),
		"createdAt":      platform.DotNetTime(n.CreatedAt).String(),
	})
}

// buildZoneSummaryFCMPayload deliberately carries none of the single-app
// deep-link keys.
func buildZoneSummaryFCMPayload(count int, zoneName, watchZoneID, tag string) (json.RawMessage, error) {
	title, body := zoneSummaryTitleBody(count, zoneName)
	return marshalFCMMessage(title, body, tag, map[string]string{
		"kind":        "zoneSummary",
		"watchZoneId": watchZoneID,
	})
}

func buildSavedSummaryFCMPayload(count int, tag string) (json.RawMessage, error) {
	title, body := savedSummaryTitleBody(count)
	return marshalFCMMessage(title, body, tag, map[string]string{
		"kind": "savedSummary",
	})
}
