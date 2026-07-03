package notifydispatch

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
)

func TestBuildAlertPayload_ZoneNotification_ThreadIDIsZoneID(t *testing.T) {
	t.Parallel()
	zoneID := "zone-1"
	n := notifications.DigestNotification{
		ID:                 "notif-1",
		ApplicationName:    "24/0001",
		WatchZoneID:        &zoneID,
		ApplicationAddress: "10 High St",
		EventType:          notifications.EventNewApplication,
		AuthorityID:        99,
		CreatedAt:          time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC),
	}

	raw, err := buildAlertPayload(n, 3)
	if err != nil {
		t.Fatalf("buildAlertPayload: %v", err)
	}

	var decoded struct {
		Aps struct {
			ThreadID string `json:"thread-id"`
			Badge    int    `json:"badge"`
		} `json:"aps"`
		ApplicationRef string `json:"applicationRef"`
		NotificationID string `json:"notificationId"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Aps.ThreadID != zoneID {
		t.Errorf("thread-id: got %q, want %q", decoded.Aps.ThreadID, zoneID)
	}
	if decoded.Aps.Badge != 3 {
		t.Errorf("badge: got %d, want 3", decoded.Aps.Badge)
	}
	// Single-app deep-link metadata must still be present (the rich single push).
	if decoded.ApplicationRef != "24/0001" || decoded.NotificationID != "notif-1" {
		t.Errorf("deep-link metadata: %+v", decoded)
	}
}

func TestBuildAlertPayload_SavedNotification_ThreadIDIsSaved(t *testing.T) {
	t.Parallel()
	n := notifications.DigestNotification{
		ID:                 "notif-2",
		ApplicationName:    "24/0002",
		WatchZoneID:        nil,
		ApplicationAddress: "1 Low St",
		EventType:          notifications.EventDecisionUpdate,
		AuthorityID:        99,
		CreatedAt:          time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC),
	}

	raw, err := buildAlertPayload(n, 0)
	if err != nil {
		t.Fatalf("buildAlertPayload: %v", err)
	}

	var decoded struct {
		Aps struct {
			ThreadID string `json:"thread-id"`
		} `json:"aps"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Aps.ThreadID != "saved" {
		t.Errorf("thread-id: got %q, want %q", decoded.Aps.ThreadID, "saved")
	}
}

func TestBuildZoneSummaryPayload_ShapeAndFields(t *testing.T) {
	t.Parallel()
	raw, err := buildZoneSummaryPayload(4, "Riverside", 7, "zone-1")
	if err != nil {
		t.Fatalf("buildZoneSummaryPayload: %v", err)
	}

	var decoded struct {
		Aps struct {
			Alert struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			} `json:"alert"`
			Sound    string `json:"sound"`
			Badge    int    `json:"badge"`
			ThreadID string `json:"thread-id"`
		} `json:"aps"`
		Kind           string  `json:"kind"`
		WatchZoneID    *string `json:"watchZoneId"`
		ApplicationRef *string `json:"applicationRef"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Aps.Alert.Title != "Planning updates near Riverside" {
		t.Errorf("title: got %q", decoded.Aps.Alert.Title)
	}
	if decoded.Aps.Alert.Body != "4 updates in this area. Tap to see them." {
		t.Errorf("body: got %q", decoded.Aps.Alert.Body)
	}
	if decoded.Aps.Badge != 7 {
		t.Errorf("badge: got %d, want 7", decoded.Aps.Badge)
	}
	if decoded.Aps.ThreadID != "zone-1" {
		t.Errorf("thread-id: got %q, want zone-1", decoded.Aps.ThreadID)
	}
	if decoded.Kind != "zoneSummary" {
		t.Errorf("kind: got %q, want zoneSummary", decoded.Kind)
	}
	if decoded.WatchZoneID == nil || *decoded.WatchZoneID != "zone-1" {
		t.Errorf("watchZoneId: got %v, want zone-1", decoded.WatchZoneID)
	}

	// A summary must never carry the single-app deep-link metadata — the absence
	// is how iOS knows to open the in-app list.
	var raw2 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &raw2); err != nil {
		t.Fatalf("unmarshal raw map: %v", err)
	}
	if _, ok := raw2["applicationRef"]; ok {
		t.Error("zone summary payload must not carry applicationRef")
	}
	if _, ok := raw2["notificationId"]; ok {
		t.Error("zone summary payload must not carry notificationId")
	}
}

func TestBuildSavedSummaryPayload_ShapeAndFields(t *testing.T) {
	t.Parallel()
	raw, err := buildSavedSummaryPayload(2, 5)
	if err != nil {
		t.Fatalf("buildSavedSummaryPayload: %v", err)
	}

	var decoded struct {
		Aps struct {
			Alert struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			} `json:"alert"`
			Badge    int    `json:"badge"`
			ThreadID string `json:"thread-id"`
		} `json:"aps"`
		Kind        string  `json:"kind"`
		WatchZoneID *string `json:"watchZoneId"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Aps.Alert.Title != "Your saved applications" {
		t.Errorf("title: got %q", decoded.Aps.Alert.Title)
	}
	if decoded.Aps.Alert.Body != "2 have a decision. Tap to see them." {
		t.Errorf("body: got %q", decoded.Aps.Alert.Body)
	}
	if decoded.Aps.Badge != 5 {
		t.Errorf("badge: got %d, want 5", decoded.Aps.Badge)
	}
	if decoded.Aps.ThreadID != "saved" {
		t.Errorf("thread-id: got %q, want saved", decoded.Aps.ThreadID)
	}
	if decoded.Kind != "savedSummary" {
		t.Errorf("kind: got %q, want savedSummary", decoded.Kind)
	}
	if decoded.WatchZoneID != nil {
		t.Errorf("saved summary must have no watchZoneId, got %v", *decoded.WatchZoneID)
	}

	var raw2 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &raw2); err != nil {
		t.Fatalf("unmarshal raw map: %v", err)
	}
	if _, ok := raw2["applicationRef"]; ok {
		t.Error("saved summary payload must not carry applicationRef")
	}
}
