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

func TestPlatformDispatcher_RoutesEachPayloadToItsSender(t *testing.T) {
	t.Parallel()
	apns := &fakePush{}
	fcm := &fakePush{}
	d := NewPlatformDispatcher(apns, fcm, testLogger(t))

	iosPayload := json.RawMessage(`{"aps":{}}`)
	androidPayload := json.RawMessage(`{"notification":{}}`)
	if _, err := d.Send(context.Background(),
		[]string{"ios-1"}, iosPayload,
		[]string{"and-1", "and-2"}, androidPayload,
	); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if apns.calls != 1 || len(apns.tokens) != 1 || apns.tokens[0] != "ios-1" {
		t.Errorf("apns should get exactly the iOS tokens, got calls=%d tokens=%v", apns.calls, apns.tokens)
	}
	if string(apns.payloads[0]) != string(iosPayload) {
		t.Errorf("apns payload = %s, want %s", apns.payloads[0], iosPayload)
	}
	if fcm.calls != 1 || len(fcm.tokens) != 2 {
		t.Errorf("fcm should get exactly the Android tokens, got calls=%d tokens=%v", fcm.calls, fcm.tokens)
	}
	if string(fcm.payloads[0]) != string(androidPayload) {
		t.Errorf("fcm payload = %s, want %s", fcm.payloads[0], androidPayload)
	}
}

func TestPlatformDispatcher_SkipsPlatformWithNoTokens(t *testing.T) {
	t.Parallel()
	apns := &fakePush{}
	fcm := &fakePush{}
	d := NewPlatformDispatcher(apns, fcm, testLogger(t))

	// Android-only recipient: APNs sender must not be touched.
	if _, err := d.Send(context.Background(), nil, nil, []string{"and-1"}, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if apns.calls != 0 {
		t.Errorf("apns must not be called with no iOS tokens, got %d", apns.calls)
	}
	if fcm.calls != 1 {
		t.Errorf("fcm should be called once, got %d", fcm.calls)
	}
}

func TestPlatformDispatcher_UnionsInvalidTokensAcrossBothSenders(t *testing.T) {
	t.Parallel()
	apns := &fakePush{invalid: []string{"ios-dead"}}
	fcm := &fakePush{invalid: []string{"and-dead"}}
	d := NewPlatformDispatcher(apns, fcm, testLogger(t))

	invalid, err := d.Send(context.Background(),
		[]string{"ios-dead"}, json.RawMessage(`{}`),
		[]string{"and-dead"}, json.RawMessage(`{}`),
	)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	got := map[string]bool{}
	for _, tok := range invalid {
		got[tok] = true
	}
	if !got["ios-dead"] || !got["and-dead"] || len(invalid) != 2 {
		t.Errorf("invalid union = %v, want [ios-dead and-dead]", invalid)
	}
}

func TestPlatformDispatcher_OnePlatformErrorDoesNotBlockOther(t *testing.T) {
	t.Parallel()
	apns := &fakePush{sendErr: errors.New("apns down")}
	fcm := &fakePush{invalid: []string{"and-dead"}}
	d := NewPlatformDispatcher(apns, fcm, testLogger(t))

	invalid, err := d.Send(context.Background(),
		[]string{"ios-1"}, json.RawMessage(`{}`),
		[]string{"and-dead"}, json.RawMessage(`{}`),
	)
	if err != nil {
		t.Fatalf("Send must swallow a per-platform error, got %v", err)
	}
	// APNs failed but FCM's invalid token is still surfaced for pruning.
	if len(invalid) != 1 || invalid[0] != "and-dead" {
		t.Errorf("invalid = %v, want [and-dead] (fcm still pruned despite apns failure)", invalid)
	}
	if fcm.calls != 1 {
		t.Errorf("fcm must still be sent to when apns fails, got %d calls", fcm.calls)
	}
}

// ---- coalescer platform-split tests ----

func newMixedCoalescer(t *testing.T) (*PushCoalescer, *fakeDevices, *fakePush, *fakePush) {
	t.Helper()
	devs := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{
		"auth0|alice": {
			{Token: "ios-1", Platform: devicetokens.PlatformIos},
			{Token: "and-1", Platform: devicetokens.PlatformAndroid},
		},
	}}
	st := &fakeState{unread: 2}
	apns := &fakePush{}
	fcm := &fakePush{}
	zones := &fakeZoneNames{byUser: map[string][]watchzones.WatchZone{}}
	disp := NewPlatformDispatcher(apns, fcm, testLogger(t))
	c := NewPushCoalescer(devs, st, disp, zones, testLogger(t))
	return c, devs, apns, fcm
}

func TestPushCoalescer_MixedRecipient_SplitsTokensByPlatform(t *testing.T) {
	t.Parallel()
	c, _, apns, fcm := newMixedCoalescer(t)
	zoneID := "zone-1"
	c.Add("auth0|alice", notifications.DigestNotification{
		ID: "n-1", ApplicationName: "24/0001", WatchZoneID: &zoneID, ApplicationAddress: "10 High St",
	})

	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// APNs sender gets the iOS token + the APNs body; FCM sender gets the Android
	// token + the FCM body — from the SAME single-item bucket.
	if apns.calls != 1 || len(apns.tokens) != 1 || apns.tokens[0] != "ios-1" {
		t.Errorf("apns tokens = %v (calls %d), want [ios-1]", apns.tokens, apns.calls)
	}
	if fcm.calls != 1 || len(fcm.tokens) != 1 || fcm.tokens[0] != "and-1" {
		t.Errorf("fcm tokens = %v (calls %d), want [and-1]", fcm.tokens, fcm.calls)
	}

	// The APNs body is the aps deep-link shape; the FCM body is the message shape.
	var apnsDecoded struct {
		Aps            map[string]json.RawMessage `json:"aps"`
		ApplicationRef string                     `json:"applicationRef"`
	}
	if err := json.Unmarshal(apns.payloads[0], &apnsDecoded); err != nil {
		t.Fatalf("apns unmarshal: %v", err)
	}
	if apnsDecoded.ApplicationRef != "24/0001" || apnsDecoded.Aps == nil {
		t.Errorf("apns payload not the alert shape: %s", apns.payloads[0])
	}
	fcmDecoded := decodeFCM(t, fcm.payloads[0])
	if fcmDecoded.Data["kind"] != "alert" || fcmDecoded.Data["applicationRef"] != "24/0001" {
		t.Errorf("fcm payload not the alert shape: %s", fcm.payloads[0])
	}
}

func TestPushCoalescer_MixedRecipient_PrunesInvalidTokensAcrossBothPlatforms(t *testing.T) {
	t.Parallel()
	c, devs, apns, fcm := newMixedCoalescer(t)
	apns.invalid = []string{"ios-1"}
	fcm.invalid = []string{"and-1"}
	c.Add("auth0|alice", notifications.DigestNotification{ID: "n-1", ApplicationAddress: "x"})

	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	deleted := map[string]bool{}
	for _, tok := range devs.deleted {
		deleted[tok] = true
	}
	if !deleted["ios-1"] || !deleted["and-1"] || len(devs.deleted) != 2 {
		t.Errorf("pruned = %v, want both ios-1 and and-1", devs.deleted)
	}
}

func TestPushCoalescer_AndroidOnlyRecipient_NoAPNsSend(t *testing.T) {
	t.Parallel()
	devs := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{
		"auth0|bob": {{Token: "and-1", Platform: devicetokens.PlatformAndroid}},
	}}
	apns := &fakePush{}
	fcm := &fakePush{}
	disp := NewPlatformDispatcher(apns, fcm, testLogger(t))
	c := NewPushCoalescer(devs, &fakeState{}, disp, &fakeZoneNames{byUser: map[string][]watchzones.WatchZone{}}, testLogger(t))
	c.Add("auth0|bob", notifications.DigestNotification{ID: "n-1", ApplicationAddress: "x"})

	if err := c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if apns.calls != 0 {
		t.Errorf("android-only recipient must not hit APNs, got %d calls", apns.calls)
	}
	if fcm.calls != 1 {
		t.Errorf("android-only recipient must hit FCM once, got %d calls", fcm.calls)
	}
}
