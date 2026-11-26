package notifydispatch

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

type webhookCall struct {
	notification notifications.DigestNotification
	app          applications.PlanningApplication
	zoneName     string
}

type fakeWebhookQueue struct {
	calls []webhookCall
	err   error
}

func (f *fakeWebhookQueue) Enqueue(_ context.Context, n notifications.DigestNotification, app applications.PlanningApplication, zoneName string) error {
	f.calls = append(f.calls, webhookCall{notification: n, app: app, zoneName: zoneName})
	return f.err
}

// webhookTiers are the tier cases shared by both dispatchers. The dispatch
// harness clock is 2026-06-13 09:00 UTC.
func webhookTiers(t *testing.T) []struct {
	name    string
	profile func() *profiles.UserProfile
	want    int
} {
	t.Helper()
	with := func(tier profiles.SubscriptionTier, expiry *time.Time) func() *profiles.UserProfile {
		return func() *profiles.UserProfile {
			p := profileWithTier(t, "auth0|alice", tier)
			p.SubscriptionExpiry = expiry
			return p
		}
	}
	lapsed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return []struct {
		name    string
		profile func() *profiles.UserProfile
		want    int
	}{
		{"pro", with(profiles.TierPro, nil), 1},
		{"personal", with(profiles.TierPersonal, nil), 0},
		{"free", with(profiles.TierFree, nil), 0},
		{"lapsed pro", with(profiles.TierPro, &lapsed), 0},
	}
}

func webhookEnqueuer(t *testing.T, profile *profiles.UserProfile, logger *slog.Logger) (*Enqueuer, *fakeNotifications) {
	t.Helper()
	notifs := newFakeNotifications()
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{profile.UserID: profile}}
	enq := NewEnqueuer(notifs, &fakeZones{}, profs, &fakePushQueue{}, &fakeDescriptionMatcher{},
		func() string { return "n-fixed" },
		func() time.Time { return time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC) },
		logger)
	return enq, notifs
}

func oldZone(t *testing.T, id string) watchzones.WatchZone {
	t.Helper()
	return testZoneAt(t, id, "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
}

func TestEnqueuer_Webhooks_QueuedOnlyForEffectivePro(t *testing.T) {
	t.Parallel()
	for _, tc := range webhookTiers(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			enq, notifs := webhookEnqueuer(t, tc.profile(), testLogger(t))
			hooks := &fakeWebhookQueue{}
			enq = enq.WithWebhooks(hooks)
			app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))

			if err := enq.Enqueue(context.Background(), app, oldZone(t, "zone-1")); err != nil {
				t.Fatalf("Enqueue: %v", err)
			}

			if len(hooks.calls) != tc.want {
				t.Fatalf("webhook calls = %d, want %d", len(hooks.calls), tc.want)
			}
			if tc.want == 0 {
				return
			}
			call := hooks.calls[0]
			if call.zoneName != "My Zone" {
				t.Errorf("zone name = %q, want My Zone", call.zoneName)
			}
			if call.notification.ID != notifs.created[0].ID || call.app.UID != app.UID {
				t.Errorf("queued notification %q application %q, want the created record and its application", call.notification.ID, call.app.UID)
			}
		})
	}
}

func TestEnqueuer_Webhooks_IgnorePushPreference(t *testing.T) {
	t.Parallel()
	profile := profileWithTier(t, "auth0|alice", profiles.TierPro)
	profile.Preferences.PushEnabled = false
	enq, _ := webhookEnqueuer(t, profile, testLogger(t))
	hooks := &fakeWebhookQueue{}
	enq = enq.WithWebhooks(hooks)

	if err := enq.Enqueue(context.Background(), testApplication(t, time.Now()), oldZone(t, "zone-1")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(hooks.calls) != 1 {
		t.Fatalf("webhook calls = %d, want 1", len(hooks.calls))
	}
}

func TestEnqueuer_Webhooks_QueueErrorIsLoggedAndSwallowed(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	enq, notifs := webhookEnqueuer(t, profileWithTier(t, "auth0|alice", profiles.TierPro),
		slog.New(slog.NewJSONHandler(&logs, nil)))
	enq = enq.WithWebhooks(&fakeWebhookQueue{err: errors.New("boom")})

	if err := enq.Enqueue(context.Background(), testApplication(t, time.Now()), oldZone(t, "zone-1")); err != nil {
		t.Fatalf("Enqueue: %v, want a webhook error to be swallowed", err)
	}
	if len(notifs.created) != 1 {
		t.Errorf("created = %d, want 1", len(notifs.created))
	}
	if !strings.Contains(logs.String(), "enqueue: webhook outbox failed") {
		t.Errorf("log = %s, want the outbox failure", logs.String())
	}
}

func TestEnqueuer_Webhooks_NotQueuedWhenCreateFails(t *testing.T) {
	t.Parallel()
	enq, notifs := webhookEnqueuer(t, profileWithTier(t, "auth0|alice", profiles.TierPro), testLogger(t))
	notifs.createErr = errors.New("db down")
	hooks := &fakeWebhookQueue{}
	enq = enq.WithWebhooks(hooks)

	if err := enq.Enqueue(context.Background(), testApplication(t, time.Now()), oldZone(t, "zone-1")); err == nil {
		t.Fatal("Enqueue: want the create error")
	}
	if len(hooks.calls) != 0 {
		t.Fatalf("webhook calls = %d, want 0", len(hooks.calls))
	}
}

func TestEnqueuer_Webhooks_NilQueueIsNoOp(t *testing.T) {
	t.Parallel()
	enq, notifs := webhookEnqueuer(t, profileWithTier(t, "auth0|alice", profiles.TierPro), testLogger(t))
	enq = enq.WithWebhooks(nil)

	if err := enq.Enqueue(context.Background(), testApplication(t, time.Now()), oldZone(t, "zone-1")); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Fatalf("created = %d, want 1", len(notifs.created))
	}
}

func decisionWithWebhooks(
	t *testing.T,
	zones []watchzones.WatchZone,
	savedUserIDs []string,
	profile *profiles.UserProfile,
	hooks webhookQueue,
) (*DecisionDispatcher, *fakeNotifications) {
	t.Helper()
	d, notifs, _ := newDecisionHarness(t,
		&fakeZones{zones: zones}, &fakeSaved{userIDs: savedUserIDs},
		&fakeProfiles{byID: map[string]*profiles.UserProfile{profile.UserID: profile}})
	return d.WithWebhooks(hooks), notifs
}

func TestDecisionDispatcher_Webhooks_QueuedOnlyForEffectivePro(t *testing.T) {
	t.Parallel()
	for _, tc := range webhookTiers(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hooks := &fakeWebhookQueue{}
			d, _ := decisionWithWebhooks(t, []watchzones.WatchZone{oldZone(t, "zone-1")}, nil, tc.profile(), hooks)

			if err := d.Dispatch(context.Background(), decisionApp(t, "Permitted", coord(51.5), coord(-0.1))); err != nil {
				t.Fatalf("Dispatch: %v", err)
			}
			if len(hooks.calls) != tc.want {
				t.Fatalf("webhook calls = %d, want %d", len(hooks.calls), tc.want)
			}
		})
	}
}

func TestDecisionDispatcher_Webhooks_ZoneName(t *testing.T) {
	t.Parallel()
	named := func(id, name string) watchzones.WatchZone {
		z := oldZone(t, id)
		z.Name = name
		return z
	}
	tests := []struct {
		name         string
		zones        []watchzones.WatchZone
		saved        []string
		wantZoneID   string
		wantZoneName string
		wantSources  string
	}{
		{"zone match", []watchzones.WatchZone{named("zone-1", "Home")}, nil, "zone-1", "Home", sourceZone},
		{
			"first of two zones gives both id and name",
			[]watchzones.WatchZone{named("zone-1", "Home"), named("zone-2", "Office")},
			nil, "zone-1", "Home", sourceZone,
		},
		{"saved only", nil, []string{"auth0|alice"}, "", "", sourceSaved},
		{
			"zone and saved",
			[]watchzones.WatchZone{named("zone-1", "Home")},
			[]string{"auth0|alice"}, "zone-1", "Home", sourceZone + "," + sourceSaved,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hooks := &fakeWebhookQueue{}
			d, _ := decisionWithWebhooks(t, tc.zones, tc.saved, profileWithTier(t, "auth0|alice", profiles.TierPro), hooks)

			if err := d.Dispatch(context.Background(), decisionApp(t, "Permitted", coord(51.5), coord(-0.1))); err != nil {
				t.Fatalf("Dispatch: %v", err)
			}
			if len(hooks.calls) != 1 {
				t.Fatalf("webhook calls = %d, want 1", len(hooks.calls))
			}
			call := hooks.calls[0]
			gotZoneID := ""
			if call.notification.WatchZoneID != nil {
				gotZoneID = *call.notification.WatchZoneID
			}
			if gotZoneID != tc.wantZoneID || call.zoneName != tc.wantZoneName || call.notification.Sources != tc.wantSources {
				t.Errorf("zone = (%q, %q) sources = %q, want (%q, %q) %q",
					gotZoneID, call.zoneName, call.notification.Sources, tc.wantZoneID, tc.wantZoneName, tc.wantSources)
			}
			if call.notification.EventType != notifications.EventDecisionUpdate {
				t.Errorf("event type = %q", call.notification.EventType)
			}
		})
	}
}

func TestDecisionDispatcher_Webhooks_IgnorePushPreferences(t *testing.T) {
	t.Parallel()
	off := profileWithTier(t, "auth0|alice", profiles.TierPro)
	off.Preferences.PushEnabled = false
	off.Preferences.SavedDecisionPush = false
	off.ZonePreferences["zone-1"] = profiles.ZonePreferences{DecisionPush: false}
	hooks := &fakeWebhookQueue{}
	d, notifs := decisionWithWebhooks(t, []watchzones.WatchZone{oldZone(t, "zone-1")}, []string{"auth0|alice"}, off, hooks)

	if err := d.Dispatch(context.Background(), decisionApp(t, "Permitted", coord(51.5), coord(-0.1))); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(hooks.calls) != 1 || notifs.created[0].PushSent {
		t.Fatalf("webhook calls = %d pushSent = %v, want 1 call and no push", len(hooks.calls), notifs.created[0].PushSent)
	}
}

func TestDecisionDispatcher_Webhooks_QueueErrorDoesNotStopFanOut(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	alice := profileWithTier(t, "auth0|alice", profiles.TierPro)
	bob := profileWithTier(t, "auth0|bob", profiles.TierPro)
	hooks := &fakeWebhookQueue{err: errors.New("boom")}
	notifs := newFakeNotifications()
	d := NewDecisionDispatcher(notifs, &fakeZones{}, &fakeSaved{userIDs: []string{alice.UserID, bob.UserID}},
		&fakeProfiles{byID: map[string]*profiles.UserProfile{alice.UserID: alice, bob.UserID: bob}},
		&fakePushQueue{},
		func() string { return "n-fixed" },
		func() time.Time { return time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC) },
		slog.New(slog.NewJSONHandler(&logs, nil))).WithWebhooks(hooks)

	if err := d.Dispatch(context.Background(), decisionApp(t, "Permitted", nil, nil)); err != nil {
		t.Fatalf("Dispatch: %v, want a webhook error to be swallowed", err)
	}
	users := make([]string, 0, len(notifs.created))
	for _, n := range notifs.created {
		users = append(users, n.UserID)
	}
	slices.Sort(users)
	if !slices.Equal(users, []string{alice.UserID, bob.UserID}) || len(hooks.calls) != 2 {
		t.Errorf("created for %v with %d webhook calls, want both users", users, len(hooks.calls))
	}
	if !strings.Contains(logs.String(), "enqueue: webhook outbox failed") {
		t.Errorf("log = %s, want the outbox failure", logs.String())
	}
}

func TestDecisionDispatcher_Webhooks_NilQueueIsNoOp(t *testing.T) {
	t.Parallel()
	d, notifs := decisionWithWebhooks(t, []watchzones.WatchZone{oldZone(t, "zone-1")}, nil,
		profileWithTier(t, "auth0|alice", profiles.TierPro), nil)

	if err := d.Dispatch(context.Background(), decisionApp(t, "Permitted", coord(51.5), coord(-0.1))); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Fatalf("created = %d, want 1", len(notifs.created))
	}
}
