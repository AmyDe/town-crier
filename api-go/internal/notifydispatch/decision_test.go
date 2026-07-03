package notifydispatch

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// fakeZones serves the cross-partition zone-containment lookup. Matching is
// purely geographic (tc-b179): the lookup no longer takes an authority, so the
// fake returns its zones regardless of which authority the app is tagged with.
type fakeZones struct {
	zones    []watchzones.WatchZone
	queryErr error
	lastLat  float64
	lastLng  float64
}

func (f *fakeZones) FindZonesContaining(_ context.Context, lat, lng float64) ([]watchzones.WatchZone, error) {
	f.lastLat = lat
	f.lastLng = lng
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.zones, nil
}

// fakeSaved serves the cross-partition saved-bookmark lookup.
type fakeSaved struct {
	userIDs  []string
	queryErr error
}

func (f *fakeSaved) UserIDsForApplication(_ context.Context, _ string, _ int) ([]string, error) {
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.userIDs, nil
}

func decisionApp(t *testing.T, decision string, lat, lng *float64) applications.PlanningApplication {
	t.Helper()
	d := decision
	app := applications.PlanningApplication{
		Name:          "24/0001",
		UID:           "24/0001",
		AreaName:      "Kingston",
		AreaID:        99,
		Address:       "10 High St",
		Description:   "Loft conversion",
		AppState:      &d,
		Latitude:      lat,
		Longitude:     lng,
		LastDifferent: time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC),
	}
	return app
}

func coord(v float64) *float64 { return &v }

func proWithZonePrefs(t *testing.T, decisionPush bool) *profiles.UserProfile {
	t.Helper()
	p := profileWithTier(t, "auth0|alice", profiles.TierPro)
	p.ZonePreferences["zone-1"] = profiles.ZonePreferences{
		NewApplicationPush: true,
		DecisionPush:       decisionPush,
	}
	return p
}

func newDecisionHarness(
	t *testing.T,
	zones *fakeZones,
	saved *fakeSaved,
	profs *fakeProfiles,
) (*DecisionDispatcher, *fakeNotifications, *fakePushQueue) {
	t.Helper()
	notifs := newFakeNotifications()
	queue := &fakePushQueue{}
	d := NewDecisionDispatcher(notifs, zones, saved, profs, queue,
		func() string { return "n-fixed" },
		func() time.Time { return time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC) },
		testLogger(t))
	return d, notifs, queue
}

func TestDecisionDispatcher_ZoneMatch_CreatesDecisionRecordAndQueuesPush(t *testing.T) {
	t.Parallel()
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{zone}}
	saved := &fakeSaved{}
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{
		"auth0|alice": proWithZonePrefs(t, true),
	}}
	d, notifs, queue := newDecisionHarness(t, zones, saved, profs)
	app := decisionApp(t, "Permitted", coord(51.5), coord(-0.1))

	if err := d.Dispatch(context.Background(), app); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Fatalf("expected 1 decision record, got %d", len(notifs.created))
	}
	rec := notifs.created[0]
	if rec.EventType != notifications.EventDecisionUpdate {
		t.Errorf("event type: got %q, want DecisionUpdate", rec.EventType)
	}
	if rec.Decision == nil || *rec.Decision != "Permitted" {
		t.Errorf("decision: got %v", rec.Decision)
	}
	if rec.WatchZoneID == nil || *rec.WatchZoneID != "zone-1" {
		t.Errorf("zone id: got %v", rec.WatchZoneID)
	}
	if rec.Sources != sourceZone {
		t.Errorf("sources: got %q, want Zone", rec.Sources)
	}
	if !rec.PushSent {
		t.Error("PushSent should be true once queued for a push-eligible user")
	}
	if len(queue.queued) != 1 {
		t.Errorf("paid tier with decision push opted in should queue exactly one push, got %d", len(queue.queued))
	}
}

func TestDecisionDispatcher_Dispatch_MatchesZoneRegardlessOfAuthority(t *testing.T) {
	t.Parallel()
	// Decision fan-out is now boundary-agnostic (tc-b179): the zone lookup is purely
	// geographic, so a zone whose circle contains the application is matched even
	// when the application is tagged a neighbouring authority. The fake returns the
	// zone for the app's coordinates; the dispatcher must dispatch to its owner.
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{zone}}
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{
		"auth0|alice": proWithZonePrefs(t, true),
	}}
	d, notifs, _ := newDecisionHarness(t, zones, &fakeSaved{}, profs)
	// App tagged a different authority (246) than the zone (99), inside the circle.
	app := decisionApp(t, "Permitted", coord(51.5), coord(-0.1))
	app.AreaID = 246

	if err := d.Dispatch(context.Background(), app); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if zones.lastLat != 51.5 || zones.lastLng != -0.1 {
		t.Errorf("zone lookup point: got lat=%v lng=%v", zones.lastLat, zones.lastLng)
	}
	if len(notifs.created) != 1 {
		t.Fatalf("cross-authority zone must still receive a decision record, got %d", len(notifs.created))
	}
	if notifs.created[0].WatchZoneID == nil || *notifs.created[0].WatchZoneID != "zone-1" {
		t.Errorf("wrong zone fanned out: %+v", notifs.created[0].WatchZoneID)
	}
}

func TestDecisionDispatcher_ZoneMatch_DecisionPushOptedOut_RecordButNoPush(t *testing.T) {
	t.Parallel()
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{zone}}
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{
		"auth0|alice": proWithZonePrefs(t, false),
	}}
	d, notifs, queue := newDecisionHarness(t, zones, &fakeSaved{}, profs)
	app := decisionApp(t, "Rejected", coord(51.5), coord(-0.1))

	if err := d.Dispatch(context.Background(), app); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Errorf("record must be written even when decision push is opted out, got %d", len(notifs.created))
	}
	if len(queue.queued) != 0 {
		t.Errorf("decision-push opt-out must suppress the queued push, got %d", len(queue.queued))
	}
}

func TestDecisionDispatcher_FreeTier_RecordNoPush(t *testing.T) {
	t.Parallel()
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{zone}}
	free := profileWithTier(t, "auth0|alice", profiles.TierFree)
	free.ZonePreferences["zone-1"] = profiles.ZonePreferences{DecisionPush: true}
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|alice": free}}
	d, notifs, queue := newDecisionHarness(t, zones, &fakeSaved{}, profs)
	app := decisionApp(t, "Permitted", coord(51.5), coord(-0.1))

	if err := d.Dispatch(context.Background(), app); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Errorf("free tier must still get the decision record, got %d", len(notifs.created))
	}
	if len(queue.queued) != 0 {
		t.Errorf("free tier must NOT queue a push, got %d", len(queue.queued))
	}
}

func TestDecisionDispatcher_ExpiredPaidTier_RecordNoPush(t *testing.T) {
	t.Parallel()
	// A Pro user opted into decision push, but whose subscription has lapsed (past
	// expiry, no grace), reads as Free: the record is written but no push fires.
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{zone}}
	lapsed := proWithZonePrefs(t, true)
	past := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) // before the harness clock
	lapsed.SubscriptionExpiry = &past
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|alice": lapsed}}
	d, notifs, queue := newDecisionHarness(t, zones, &fakeSaved{}, profs)
	app := decisionApp(t, "Permitted", coord(51.5), coord(-0.1))

	if err := d.Dispatch(context.Background(), app); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Errorf("lapsed paid tier must still get the decision record, got %d", len(notifs.created))
	}
	if len(queue.queued) != 0 {
		t.Errorf("lapsed paid tier must NOT queue a push, got %d", len(queue.queued))
	}
}

func TestDecisionDispatcher_SavedOnly_NilZoneSourcesSaved(t *testing.T) {
	t.Parallel()
	zones := &fakeZones{}
	saved := &fakeSaved{userIDs: []string{"auth0|bob"}}
	bob := profileWithTier(t, "auth0|bob", profiles.TierPro)
	bob.Preferences.SavedDecisionPush = true
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|bob": bob}}
	d, notifs, queue := newDecisionHarness(t, zones, saved, profs)
	app := decisionApp(t, "Permitted", coord(51.5), coord(-0.1))

	if err := d.Dispatch(context.Background(), app); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Fatalf("expected 1 saved-decision record, got %d", len(notifs.created))
	}
	rec := notifs.created[0]
	if rec.WatchZoneID != nil {
		t.Errorf("saved-only record must have nil zone id, got %v", *rec.WatchZoneID)
	}
	if rec.Sources != sourceSaved {
		t.Errorf("sources: got %q, want Saved", rec.Sources)
	}
	if len(queue.queued) != 1 {
		t.Errorf("saved-decision push opted in should queue exactly one push, got %d", len(queue.queued))
	}
}

func TestDecisionDispatcher_ZoneAndSaved_MergesSources(t *testing.T) {
	t.Parallel()
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{zone}}
	saved := &fakeSaved{userIDs: []string{"auth0|alice"}}
	alice := proWithZonePrefs(t, true)
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|alice": alice}}
	d, notifs, _ := newDecisionHarness(t, zones, saved, profs)
	app := decisionApp(t, "Permitted", coord(51.5), coord(-0.1))

	if err := d.Dispatch(context.Background(), app); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Fatalf("a user matched by both zone and saved must get ONE merged record, got %d", len(notifs.created))
	}
	rec := notifs.created[0]
	if rec.Sources != sourceZone+","+sourceSaved {
		t.Errorf("merged sources: got %q, want %q", rec.Sources, sourceZone+","+sourceSaved)
	}
	if rec.WatchZoneID == nil || *rec.WatchZoneID != "zone-1" {
		t.Errorf("merged record should attribute the zone id: %v", rec.WatchZoneID)
	}
}

func TestDecisionDispatcher_Idempotent_SkipsExisting(t *testing.T) {
	t.Parallel()
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{zone}}
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{
		"auth0|alice": proWithZonePrefs(t, true),
	}}
	d, notifs, queue := newDecisionHarness(t, zones, &fakeSaved{}, profs)
	app := decisionApp(t, "Permitted", coord(51.5), coord(-0.1))

	if err := d.Dispatch(context.Background(), app); err != nil {
		t.Fatalf("first Dispatch: %v", err)
	}
	if err := d.Dispatch(context.Background(), app); err != nil {
		t.Fatalf("second Dispatch: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Errorf("re-dispatch must not double-create, got %d records", len(notifs.created))
	}
	if len(queue.queued) != 1 {
		t.Errorf("re-dispatch must not double-queue a push, got %d", len(queue.queued))
	}
}

func TestDecisionDispatcher_NoCoords_OnlySavedFanOut(t *testing.T) {
	t.Parallel()
	zones := &fakeZones{zones: []watchzones.WatchZone{
		testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)),
	}}
	saved := &fakeSaved{userIDs: []string{"auth0|bob"}}
	bob := profileWithTier(t, "auth0|bob", profiles.TierPro)
	bob.Preferences.SavedDecisionPush = true
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|bob": bob}}
	d, notifs, _ := newDecisionHarness(t, zones, saved, profs)
	app := decisionApp(t, "Permitted", nil, nil)

	if err := d.Dispatch(context.Background(), app); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	// No coordinates means the zone lookup must be skipped entirely; only the
	// saved bookmark holder is notified.
	if zones.lastLat != 0 || zones.lastLng != 0 {
		t.Errorf("zone lookup should not run without coordinates")
	}
	if len(notifs.created) != 1 || notifs.created[0].UserID != "auth0|bob" {
		t.Errorf("expected only the saved holder notified, got %+v", notifs.created)
	}
}
