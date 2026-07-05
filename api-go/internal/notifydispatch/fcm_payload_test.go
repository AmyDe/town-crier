package notifydispatch

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// decodeFCM unpacks a token-less FCM message body for assertions.
func decodeFCM(t *testing.T, raw json.RawMessage) struct {
	Notification struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	} `json:"notification"`
	Android struct {
		Priority     string `json:"priority"`
		Notification struct {
			ChannelID string `json:"channel_id"`
		} `json:"notification"`
	} `json:"android"`
	Data map[string]string `json:"data"`
} {
	t.Helper()
	var decoded struct {
		Notification struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		} `json:"notification"`
		Android struct {
			Priority     string `json:"priority"`
			Notification struct {
				ChannelID string `json:"channel_id"`
			} `json:"notification"`
		} `json:"android"`
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal fcm payload: %v", err)
	}
	return decoded
}

func TestBuildAlertFCMPayload_CarriesDeepLinkDataAsStrings(t *testing.T) {
	t.Parallel()
	zoneID := "zone-1"
	created := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	n := notifications.DigestNotification{
		ID:                 "notif-1",
		ApplicationName:    "24/0001",
		WatchZoneID:        &zoneID,
		ApplicationAddress: "10 High St",
		EventType:          notifications.EventNewApplication,
		AuthorityID:        99,
		CreatedAt:          created,
	}

	raw, err := buildAlertFCMPayload(n)
	if err != nil {
		t.Fatalf("buildAlertFCMPayload: %v", err)
	}
	decoded := decodeFCM(t, raw)

	if decoded.Notification.Title != "Planning update near you" {
		t.Errorf("title: got %q", decoded.Notification.Title)
	}
	if decoded.Notification.Body != "10 High St" {
		t.Errorf("body: got %q", decoded.Notification.Body)
	}
	if decoded.Android.Priority != "HIGH" {
		t.Errorf("android priority: got %q, want HIGH", decoded.Android.Priority)
	}
	if decoded.Android.Notification.ChannelID != "alerts" {
		t.Errorf("channel_id: got %q, want alerts", decoded.Android.Notification.ChannelID)
	}
	if decoded.Data["kind"] != "alert" {
		t.Errorf("data.kind: got %q, want alert", decoded.Data["kind"])
	}
	if decoded.Data["notificationId"] != "notif-1" {
		t.Errorf("data.notificationId: got %q", decoded.Data["notificationId"])
	}
	if decoded.Data["applicationRef"] != "24/0001" {
		t.Errorf("data.applicationRef: got %q", decoded.Data["applicationRef"])
	}
	if decoded.Data["authorityId"] != "99" {
		t.Errorf("data.authorityId: got %q, want stringified 99", decoded.Data["authorityId"])
	}
	wantCreated := platform.DotNetTime(created).String()
	if decoded.Data["createdAt"] != wantCreated {
		t.Errorf("data.createdAt: got %q, want %q", decoded.Data["createdAt"], wantCreated)
	}

	// No badge field anywhere (Android badges are channel-driven).
	if _, ok := decoded.Data["badge"]; ok {
		t.Error("fcm data must not carry a badge")
	}
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		t.Fatalf("unmarshal raw map: %v", err)
	}
	if _, ok := rawMap["badge"]; ok {
		t.Error("fcm message must not carry a top-level badge")
	}
}

func TestBuildAlertFCMPayload_SavedNotificationTitle(t *testing.T) {
	t.Parallel()
	n := notifications.DigestNotification{
		ID:                 "notif-2",
		ApplicationName:    "24/0002",
		WatchZoneID:        nil,
		ApplicationAddress: "1 Low St",
		EventType:          notifications.EventDecisionUpdate,
		AuthorityID:        5,
		CreatedAt:          time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC),
	}

	raw, err := buildAlertFCMPayload(n)
	if err != nil {
		t.Fatalf("buildAlertFCMPayload: %v", err)
	}
	decoded := decodeFCM(t, raw)
	if decoded.Notification.Title != "Town Crier" {
		t.Errorf("saved-only title: got %q, want Town Crier", decoded.Notification.Title)
	}
}

func TestBuildZoneSummaryFCMPayload_KindAndWatchZoneNoDeepLink(t *testing.T) {
	t.Parallel()
	raw, err := buildZoneSummaryFCMPayload(4, "Riverside", "zone-1")
	if err != nil {
		t.Fatalf("buildZoneSummaryFCMPayload: %v", err)
	}
	decoded := decodeFCM(t, raw)

	if decoded.Notification.Title != "Planning updates near Riverside" {
		t.Errorf("title: got %q", decoded.Notification.Title)
	}
	if decoded.Notification.Body != "4 updates in this area. Tap to see them." {
		t.Errorf("body: got %q", decoded.Notification.Body)
	}
	if decoded.Data["kind"] != "zoneSummary" {
		t.Errorf("data.kind: got %q, want zoneSummary", decoded.Data["kind"])
	}
	if decoded.Data["watchZoneId"] != "zone-1" {
		t.Errorf("data.watchZoneId: got %q, want zone-1", decoded.Data["watchZoneId"])
	}
	// A summary must never carry the single-app deep-link keys.
	if _, ok := decoded.Data["applicationRef"]; ok {
		t.Error("zone summary must not carry applicationRef")
	}
	if _, ok := decoded.Data["notificationId"]; ok {
		t.Error("zone summary must not carry notificationId")
	}
}

func TestBuildSavedSummaryFCMPayload_KindNoWatchZoneNoDeepLink(t *testing.T) {
	t.Parallel()
	raw, err := buildSavedSummaryFCMPayload(2)
	if err != nil {
		t.Fatalf("buildSavedSummaryFCMPayload: %v", err)
	}
	decoded := decodeFCM(t, raw)

	if decoded.Notification.Title != "Your saved applications" {
		t.Errorf("title: got %q", decoded.Notification.Title)
	}
	if decoded.Notification.Body != "2 have a decision. Tap to see them." {
		t.Errorf("body: got %q", decoded.Notification.Body)
	}
	if decoded.Data["kind"] != "savedSummary" {
		t.Errorf("data.kind: got %q, want savedSummary", decoded.Data["kind"])
	}
	if _, ok := decoded.Data["watchZoneId"]; ok {
		t.Error("saved summary must not carry watchZoneId")
	}
	if _, ok := decoded.Data["applicationRef"]; ok {
		t.Error("saved summary must not carry applicationRef")
	}
}

// TestFCMAndAPNsShareCopy asserts the two platforms render identical title/body
// strings for the same event, which is the whole point of the shared helpers.
func TestFCMAndAPNsShareCopy(t *testing.T) {
	t.Parallel()
	zoneID := "zone-1"
	n := notifications.DigestNotification{
		ID: "n-1", ApplicationName: "24/0001", WatchZoneID: &zoneID,
		ApplicationAddress: "10 High St", EventType: notifications.EventNewApplication,
		AuthorityID: 1, CreatedAt: time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC),
	}

	apnsRaw, err := buildAlertPayload(n, 3)
	if err != nil {
		t.Fatalf("buildAlertPayload: %v", err)
	}
	fcmRaw, err := buildAlertFCMPayload(n)
	if err != nil {
		t.Fatalf("buildAlertFCMPayload: %v", err)
	}

	var apnsDecoded struct {
		Aps struct {
			Alert struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			} `json:"alert"`
		} `json:"aps"`
	}
	if err := json.Unmarshal(apnsRaw, &apnsDecoded); err != nil {
		t.Fatalf("unmarshal apns: %v", err)
	}
	fcm := decodeFCM(t, fcmRaw)
	if fcm.Notification.Title != apnsDecoded.Aps.Alert.Title {
		t.Errorf("titles differ: fcm %q vs apns %q", fcm.Notification.Title, apnsDecoded.Aps.Alert.Title)
	}
	if fcm.Notification.Body != apnsDecoded.Aps.Alert.Body {
		t.Errorf("bodies differ: fcm %q vs apns %q", fcm.Notification.Body, apnsDecoded.Aps.Alert.Body)
	}
}
