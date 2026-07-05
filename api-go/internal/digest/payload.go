package digest

import (
	"encoding/json"
	"fmt"
)

// apnsDigestPayload is the APNs body for a digest push: {"aps":{...}}. The JSON
// keys follow the APNs aps dictionary spec the iOS client expects.
type apnsDigestPayload struct {
	Aps apnsDigestAps `json:"aps"`
}

type apnsDigestAps struct {
	Alert apnsAlertContent `json:"alert"`
	Sound string           `json:"sound"`
	Badge int              `json:"badge"`
}

type apnsAlertContent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// digestTitle is the title both the APNs and FCM digest pushes carry.
const digestTitle = "Town Crier"

// digestBody renders the digest push body copy ("N new application(s) this
// week"). Shared by the APNs and FCM builders so the two platforms render
// byte-identical copy for the same digest.
func digestBody(applicationCount int) string {
	plural := "s"
	if applicationCount == 1 {
		plural = ""
	}
	return fmt.Sprintf("%d new application%s this week", applicationCount, plural)
}

// buildDigestPayload renders the APNs digest push body. applicationCount is the
// number of applications in the digest window (rendered in the body copy);
// totalUnreadCount is the user's total unread tally surfaced as the app icon
// badge — the two are deliberately distinct.
func buildDigestPayload(applicationCount, totalUnreadCount int) (json.RawMessage, error) {
	raw, err := json.Marshal(apnsDigestPayload{
		Aps: apnsDigestAps{
			Alert: apnsAlertContent{Title: digestTitle, Body: digestBody(applicationCount)},
			Sound: "default",
			Badge: totalUnreadCount,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal digest payload: %w", err)
	}
	return raw, nil
}

// fcmDigestMessage is the FCM HTTP v1 "message" object for the weekly digest
// push, minus the per-recipient token (fcm.Client injects it). It mirrors the
// APNs digest shape: a system-rendered notification with the same title/body,
// plus a data dictionary carrying kind=digest (the #777 routing discriminator).
// A digest push opens the app, so it carries no deep-link keys; there is no
// badge field (Android badges are channel-driven).
type fcmDigestMessage struct {
	Notification fcmDigestNotification `json:"notification"`
	Android      fcmDigestAndroid      `json:"android"`
	Data         map[string]string     `json:"data"`
}

type fcmDigestNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type fcmDigestAndroid struct {
	Priority     string                       `json:"priority"`
	Notification fcmDigestAndroidNotification `json:"notification"`
}

type fcmDigestAndroidNotification struct {
	ChannelID string `json:"channel_id"`
}

// buildDigestFCMPayload renders the weekly digest FCM push body, carrying the
// same title/body copy as the APNs digest push.
func buildDigestFCMPayload(applicationCount int) (json.RawMessage, error) {
	raw, err := json.Marshal(fcmDigestMessage{
		Notification: fcmDigestNotification{Title: digestTitle, Body: digestBody(applicationCount)},
		Android: fcmDigestAndroid{
			Priority:     "HIGH",
			Notification: fcmDigestAndroidNotification{ChannelID: "alerts"},
		},
		Data: map[string]string{"kind": "digest"},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal digest fcm payload: %w", err)
	}
	return raw, nil
}
