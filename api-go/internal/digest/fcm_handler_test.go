package digest

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

func TestBuildDigestFCMPayload_Shape(t *testing.T) {
	t.Parallel()
	raw, err := buildDigestFCMPayload(3)
	if err != nil {
		t.Fatalf("buildDigestFCMPayload: %v", err)
	}
	var parsed struct {
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
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Notification.Title != "Town Crier" {
		t.Errorf("title: got %q", parsed.Notification.Title)
	}
	if parsed.Notification.Body != "3 new applications this week" {
		t.Errorf("body: got %q", parsed.Notification.Body)
	}
	if parsed.Android.Notification.ChannelID != "alerts" {
		t.Errorf("channel_id: got %q", parsed.Android.Notification.ChannelID)
	}
	if parsed.Data["kind"] != "digest" {
		t.Errorf("data.kind: got %q, want digest", parsed.Data["kind"])
	}
	// A digest push carries no deep-link keys and no badge.
	if _, ok := parsed.Data["badge"]; ok {
		t.Error("fcm digest must not carry a badge")
	}
	if _, ok := parsed.Data["notificationId"]; ok {
		t.Error("fcm digest must not carry deep-link keys")
	}
}

func TestRunWeekly_MixedRecipient_APNsToIosAndFCMToAndroid(t *testing.T) {
	t.Parallel()
	prefs := profiles.DefaultPreferences()
	prefs.EmailDigestEnabled = false
	prefs.PushEnabled = true
	p := mkProfile(t, profiles.TierPro, prefs)

	fp := &fakeProfiles{byDay: map[time.Weekday][]*profiles.UserProfile{time.Wednesday: {p}}}
	fn := &fakeNotifications{sinceByUser: map[string][]notifications.DigestNotification{
		"user-1": {zoneNotif("uid-A", "zone-1"), zoneNotif("uid-B", "zone-1")},
	}}
	state := &fakeState{unread: map[string]int{"user-1": 5}}
	devices := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{
		"user-1": {
			{UserID: "user-1", Token: "ios-tok", Platform: devicetokens.PlatformIos},
			{UserID: "user-1", Token: "and-tok", Platform: devicetokens.PlatformAndroid},
		},
	}}
	ios := &spyPush{}
	android := &spyPush{}
	h := newHandlerWithDispatcher(fp, fn, &fakeZones{}, state, devices, &spyEmail{}, &fakeDispatcher{ios: ios, android: android})

	if err := h.RunWeekly(context.Background()); err != nil {
		t.Fatalf("RunWeekly: %v", err)
	}

	if ios.calls != 1 || len(ios.tokens) != 1 || ios.tokens[0] != "ios-tok" {
		t.Errorf("apns should get the iOS token, got calls=%d tokens=%v", ios.calls, ios.tokens)
	}
	if android.calls != 1 || len(android.tokens) != 1 || android.tokens[0] != "and-tok" {
		t.Errorf("fcm should get the Android token, got calls=%d tokens=%v", android.calls, android.tokens)
	}

	// APNs body: aps digest shape with the badge and rendered body.
	var apnsParsed struct {
		Aps struct {
			Badge int `json:"badge"`
			Alert struct {
				Body string `json:"body"`
			} `json:"alert"`
		} `json:"aps"`
	}
	if err := json.Unmarshal(ios.payload, &apnsParsed); err != nil {
		t.Fatalf("apns unmarshal: %v", err)
	}
	if apnsParsed.Aps.Badge != 5 {
		t.Errorf("apns badge: got %d, want 5", apnsParsed.Aps.Badge)
	}
	if apnsParsed.Aps.Alert.Body != "2 new applications this week" {
		t.Errorf("apns body: got %q", apnsParsed.Aps.Alert.Body)
	}

	// FCM body: message shape, kind=digest, same body copy, no badge.
	var fcmParsed struct {
		Notification struct {
			Body string `json:"body"`
		} `json:"notification"`
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(android.payload, &fcmParsed); err != nil {
		t.Fatalf("fcm unmarshal: %v", err)
	}
	if fcmParsed.Data["kind"] != "digest" {
		t.Errorf("fcm kind: got %q, want digest", fcmParsed.Data["kind"])
	}
	if fcmParsed.Notification.Body != "2 new applications this week" {
		t.Errorf("fcm body: got %q", fcmParsed.Notification.Body)
	}
}

func TestRunWeekly_AndroidOnlyRecipient_PrunesInvalidFCMTokenNoAPNs(t *testing.T) {
	t.Parallel()
	prefs := profiles.DefaultPreferences()
	prefs.EmailDigestEnabled = false
	prefs.PushEnabled = true
	p := mkProfile(t, profiles.TierPro, prefs)

	fp := &fakeProfiles{byDay: map[time.Weekday][]*profiles.UserProfile{time.Wednesday: {p}}}
	fn := &fakeNotifications{sinceByUser: map[string][]notifications.DigestNotification{
		"user-1": {zoneNotif("uid-A", "zone-1")},
	}}
	devices := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{
		"user-1": {{UserID: "user-1", Token: "and-dead", Platform: devicetokens.PlatformAndroid}},
	}}
	ios := &spyPush{}
	android := &spyPush{invalid: []string{"and-dead"}}
	h := newHandlerWithDispatcher(fp, fn, &fakeZones{}, &fakeState{unread: map[string]int{}}, devices, &spyEmail{}, &fakeDispatcher{ios: ios, android: android})

	if err := h.RunWeekly(context.Background()); err != nil {
		t.Fatalf("RunWeekly: %v", err)
	}
	if ios.calls != 0 {
		t.Errorf("android-only recipient must not hit APNs, got %d", ios.calls)
	}
	if android.calls != 1 {
		t.Errorf("android-only recipient must hit FCM once, got %d", android.calls)
	}
	if len(devices.deleted) != 1 || devices.deleted[0] != "and-dead" {
		t.Errorf("pruned = %v, want [and-dead]", devices.deleted)
	}
}
