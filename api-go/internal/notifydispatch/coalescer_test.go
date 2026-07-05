package notifydispatch

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// fakeDevices serves device tokens and records pruned ones. listCalls counts
// ListByUser invocations per user so the tests can assert devices are loaded
// exactly once per flushed user.
type fakeDevices struct {
	byUser    map[string][]devicetokens.DeviceRegistration
	deleted   []string
	listCalls map[string]int
}

func (f *fakeDevices) ListByUser(_ context.Context, userID string) ([]devicetokens.DeviceRegistration, error) {
	if f.listCalls == nil {
		f.listCalls = map[string]int{}
	}
	f.listCalls[userID]++
	return f.byUser[userID], nil
}

func (f *fakeDevices) Delete(_ context.Context, _, token string) error {
	f.deleted = append(f.deleted, token)
	return nil
}

// fakeState serves the unread count (read_at IS NULL, ADR 0035).
type fakeState struct {
	unread int
}

func (f *fakeState) UnreadCount(_ context.Context, _ string) (int, error) {
	return f.unread, nil
}

// fakePush records the payloads it was asked to send and which devices it
// returns as invalid. sendErr, when set, is returned instead so tests can
// assert a send failure is swallowed rather than propagated.
type fakePush struct {
	calls    int
	tokens   []string
	payloads []json.RawMessage
	invalid  []string
	sendErr  error
}

func (f *fakePush) Send(_ context.Context, tokens []string, payload json.RawMessage) ([]string, error) {
	f.calls++
	f.tokens = append(f.tokens, tokens...)
	f.payloads = append(f.payloads, payload)
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	return f.invalid, nil
}

// fakeZoneNames serves the zone list the coalescer resolves names from at
// flush time, counting calls per user so tests can assert "loaded once".
type fakeZoneNames struct {
	byUser map[string][]watchzones.WatchZone
	calls  map[string]int
}

func (f *fakeZoneNames) GetByUserID(_ context.Context, userID string) ([]watchzones.WatchZone, error) {
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[userID]++
	return f.byUser[userID], nil
}

func newCoalescerHarness(t *testing.T) (*PushCoalescer, *fakeDevices, *fakeState, *fakePush, *fakeZoneNames) {
	t.Helper()
	devs := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{
		"auth0|alice": {{Token: "tok-1"}},
	}}
	st := &fakeState{unread: 2}
	push := &fakePush{}
	zones := &fakeZoneNames{byUser: map[string][]watchzones.WatchZone{}}
	// The default device is iOS (zero-value platform), so the dispatcher routes to
	// the APNs fake (push) that the existing assertions inspect. A throwaway FCM
	// fake stands in for the Android sender the iOS-only fixtures never exercise.
	disp := NewPlatformDispatcher(push, &fakePush{}, testLogger(t))
	c := NewPushCoalescer(devs, st, disp, zones, testLogger(t))
	return c, devs, st, push, zones
}

func decodeAps(t *testing.T, raw json.RawMessage) struct {
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
	ApplicationRef string  `json:"applicationRef"`
	NotificationID string  `json:"notificationId"`
} {
	t.Helper()
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
		ApplicationRef string  `json:"applicationRef"`
		NotificationID string  `json:"notificationId"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return decoded
}

func TestPushCoalescer_SingleItemBucket_SendsRichPayload(t *testing.T) {
	t.Parallel()
	c, _, _, push, _ := newCoalescerHarness(t)
	zoneID := "zone-1"
	c.Add("auth0|alice", notifications.DigestNotification{
		ID: "n-1", ApplicationName: "24/0001", WatchZoneID: &zoneID, ApplicationAddress: "10 High St",
	})

	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if push.calls != 1 {
		t.Fatalf("single-item bucket must send exactly one push, got %d", push.calls)
	}
	decoded := decodeAps(t, push.payloads[0])
	if decoded.ApplicationRef != "24/0001" || decoded.NotificationID != "n-1" {
		t.Errorf("single-item bucket must keep the rich deep-link payload: %+v", decoded)
	}
	if decoded.Aps.ThreadID != "zone-1" {
		t.Errorf("thread-id: got %q, want zone-1", decoded.Aps.ThreadID)
	}
}

func TestPushCoalescer_MultiItemZoneBucket_SendsSummaryWithZoneName(t *testing.T) {
	t.Parallel()
	c, _, _, push, zones := newCoalescerHarness(t)
	zones.byUser["auth0|alice"] = []watchzones.WatchZone{{ID: "zone-1", UserID: "auth0|alice", Name: "Riverside"}}
	zoneID := "zone-1"
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-1", WatchZoneID: &zoneID})
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-2", WatchZoneID: &zoneID})

	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if push.calls != 1 {
		t.Fatalf("two items in one zone bucket must send exactly one push, got %d", push.calls)
	}
	decoded := decodeAps(t, push.payloads[0])
	if decoded.Aps.Alert.Title != "Planning updates near Riverside" {
		t.Errorf("title: got %q", decoded.Aps.Alert.Title)
	}
	if decoded.Aps.ThreadID != "zone-1" {
		t.Errorf("thread-id: got %q, want zone-1", decoded.Aps.ThreadID)
	}
	if decoded.Kind != "zoneSummary" {
		t.Errorf("kind: got %q, want zoneSummary", decoded.Kind)
	}
	if decoded.ApplicationRef != "" {
		t.Errorf("summary payload must not carry applicationRef, got %q", decoded.ApplicationRef)
	}
}

func TestPushCoalescer_MultipleZones_OneSendEachWithNameAndThreadID(t *testing.T) {
	t.Parallel()
	c, _, _, push, zones := newCoalescerHarness(t)
	zones.byUser["auth0|alice"] = []watchzones.WatchZone{
		{ID: "zone-1", UserID: "auth0|alice", Name: "Riverside"},
		{ID: "zone-2", UserID: "auth0|alice", Name: "Hilltop"},
	}
	z1, z2 := "zone-1", "zone-2"
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-1", WatchZoneID: &z1})
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-2", WatchZoneID: &z1})
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-3", WatchZoneID: &z2})
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-4", WatchZoneID: &z2})

	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if push.calls != 2 {
		t.Fatalf("two zones must send exactly two pushes, got %d", push.calls)
	}
	gotTitles := map[string]bool{}
	for _, raw := range push.payloads {
		decoded := decodeAps(t, raw)
		if decoded.WatchZoneID == nil {
			t.Fatalf("expected watchZoneId on a zone summary push")
		}
		if decoded.Aps.ThreadID != *decoded.WatchZoneID {
			t.Errorf("thread-id must equal watchZoneId: %+v", decoded)
		}
		gotTitles[decoded.Aps.Alert.Title] = true
	}
	if !gotTitles["Planning updates near Riverside"] || !gotTitles["Planning updates near Hilltop"] {
		t.Errorf("expected one send per zone with its own name, got titles %+v", gotTitles)
	}
}

func TestPushCoalescer_SavedBucket_ThreadIDIsSaved(t *testing.T) {
	t.Parallel()
	c, _, _, push, _ := newCoalescerHarness(t)
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-1"})
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-2"})

	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if push.calls != 1 {
		t.Fatalf("saved bucket must send exactly one push, got %d", push.calls)
	}
	decoded := decodeAps(t, push.payloads[0])
	if decoded.Aps.ThreadID != "saved" {
		t.Errorf("thread-id: got %q, want saved", decoded.Aps.ThreadID)
	}
	if decoded.Kind != "savedSummary" {
		t.Errorf("kind: got %q, want savedSummary", decoded.Kind)
	}
}

func TestPushCoalescer_DevicesListedOnceAndInvalidTokenUnionPrunedOnce(t *testing.T) {
	t.Parallel()
	c, devs, _, push, zones := newCoalescerHarness(t)
	zones.byUser["auth0|alice"] = []watchzones.WatchZone{
		{ID: "zone-1", UserID: "auth0|alice", Name: "Riverside"},
		{ID: "zone-2", UserID: "auth0|alice", Name: "Hilltop"},
	}
	push.invalid = []string{"tok-1"} // every send reports the same stale token
	z1, z2 := "zone-1", "zone-2"
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-1", WatchZoneID: &z1})
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-2", WatchZoneID: &z2})

	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if devs.listCalls["auth0|alice"] != 1 {
		t.Errorf("devices must be listed exactly once per user, got %d", devs.listCalls["auth0|alice"])
	}
	if len(devs.deleted) != 1 || devs.deleted[0] != "tok-1" {
		t.Errorf("invalid token union must be pruned exactly once, got %v", devs.deleted)
	}
}

func TestPushCoalescer_NoDevices_NoSend(t *testing.T) {
	t.Parallel()
	c, _, _, push, _ := newCoalescerHarness(t)
	c.Add("auth0|nodevices", notifications.DigestNotification{ID: "n-1"})

	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if push.calls != 0 {
		t.Errorf("user with no devices must not be sent to, got %d calls", push.calls)
	}
}

func TestPushCoalescer_SendErrorIsSwallowed(t *testing.T) {
	t.Parallel()
	c, _, _, push, _ := newCoalescerHarness(t)
	push.sendErr = errors.New("apns down")
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-1"})

	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush must swallow a send failure, got %v", err)
	}
}

func TestPushCoalescer_Badge_NoPlusOne(t *testing.T) {
	t.Parallel()
	c, _, st, push, _ := newCoalescerHarness(t)
	st.unread = 4
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-1"})

	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	decoded := decodeAps(t, push.payloads[0])
	if decoded.Aps.Badge != 4 {
		t.Errorf("badge: got %d, want 4 (no +1 fudge)", decoded.Aps.Badge)
	}
}

func TestPushCoalescer_Reset_ClearsAccumulator(t *testing.T) {
	t.Parallel()
	c, _, _, push, _ := newCoalescerHarness(t)
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-1"})
	c.Reset()

	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if push.calls != 0 {
		t.Errorf("Reset must clear queued items, got %d sends", push.calls)
	}
}

func TestPushCoalescer_Flush_ConsumesQueuedItems(t *testing.T) {
	t.Parallel()
	c, _, _, push, _ := newCoalescerHarness(t)
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-1"})
	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("first Flush: %v", err)
	}
	if push.calls != 1 {
		t.Fatalf("first flush: got %d calls, want 1", push.calls)
	}
	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("second Flush: %v", err)
	}
	if push.calls != 1 {
		t.Errorf("second flush with nothing queued must not resend, got %d", push.calls)
	}
}
