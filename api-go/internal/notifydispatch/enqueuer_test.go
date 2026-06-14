package notifydispatch

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeNotifications records created notifications and serves dedup lookups from
// an in-memory map keyed by (userId, uid, authorityId, eventType).
type fakeNotifications struct {
	created   []notifications.DigestNotification
	existing  map[string]notifications.DigestNotification
	createErr error
}

func newFakeNotifications() *fakeNotifications {
	return &fakeNotifications{existing: map[string]notifications.DigestNotification{}}
}

func dedupKey(userID, uid string, authorityID int, eventType notifications.EventType) string {
	return userID + "|" + uid + "|" + string(eventType) + "|" + strconv.Itoa(authorityID)
}

func (f *fakeNotifications) Create(_ context.Context, n notifications.DigestNotification) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.created = append(f.created, n)
	f.existing[dedupKey(n.UserID, n.ApplicationUID, n.AuthorityID, n.EventType)] = n
	return nil
}

func (f *fakeNotifications) GetByUserAndApplication(_ context.Context, userID, uid string, authorityID int, eventType notifications.EventType) (*notifications.DigestNotification, error) {
	if n, ok := f.existing[dedupKey(userID, uid, authorityID, eventType)]; ok {
		return &n, nil
	}
	return nil, nil
}

// fakeProfiles serves user profiles by id.
type fakeProfiles struct {
	byID map[string]*profiles.UserProfile
}

func (f *fakeProfiles) Get(_ context.Context, userID string) (*profiles.UserProfile, error) {
	if p, ok := f.byID[userID]; ok {
		return p, nil
	}
	return nil, profiles.ErrNotFound
}

// fakeDevices serves device tokens and records pruned ones.
type fakeDevices struct {
	byUser  map[string][]devicetokens.DeviceRegistration
	deleted []string
}

func (f *fakeDevices) ListByUser(_ context.Context, userID string) ([]devicetokens.DeviceRegistration, error) {
	return f.byUser[userID], nil
}

func (f *fakeDevices) Delete(_ context.Context, _, token string) error {
	f.deleted = append(f.deleted, token)
	return nil
}

// fakeState serves the read-watermark and unread count.
type fakeState struct {
	state  *notificationstate.State
	unread int
}

func (f *fakeState) Get(_ context.Context, _ string) (*notificationstate.State, error) {
	return f.state, nil
}

func (f *fakeState) UnreadCount(_ context.Context, _ string, _ time.Time) (int, error) {
	return f.unread, nil
}

// fakePush records the payloads it was asked to send and which devices it
// returns as invalid.
type fakePush struct {
	calls    int
	tokens   []string
	payloads []json.RawMessage
	invalid  []string
}

func (f *fakePush) Send(_ context.Context, tokens []string, payload json.RawMessage) ([]string, error) {
	f.calls++
	f.tokens = append(f.tokens, tokens...)
	f.payloads = append(f.payloads, payload)
	return f.invalid, nil
}

func profileWithTier(t *testing.T, userID string, tier profiles.SubscriptionTier) *profiles.UserProfile {
	t.Helper()
	p, err := profiles.NewProfile(userID, "", time.Now())
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	p.Tier = tier
	return p
}

func testApplication(t *testing.T, areaID int, name string, lat, lng float64, lastDifferent time.Time) applications.PlanningApplication {
	t.Helper()
	la, lo := lat, lng
	state := "Pending"
	return applications.PlanningApplication{
		Name:          name,
		UID:           name,
		AreaName:      "Kingston",
		AreaID:        areaID,
		Address:       "10 High St",
		Description:   "Loft conversion",
		AppState:      &state,
		Latitude:      &la,
		Longitude:     &lo,
		LastDifferent: lastDifferent,
	}
}

func testZoneAt(t *testing.T, id, userID string, createdAt time.Time) watchzones.WatchZone {
	t.Helper()
	z, err := watchzones.NewWatchZone(id, userID, "My Zone", 51.5, -0.1, 500, 99, createdAt, true, true)
	if err != nil {
		t.Fatalf("NewWatchZone: %v", err)
	}
	return z
}

func newEnqueuerHarness(t *testing.T, tier profiles.SubscriptionTier) (*Enqueuer, *fakeNotifications, *fakePush) {
	t.Helper()
	enq, notifs, push, _ := newEnqueuerHarnessWithZones(t, tier, nil)
	return enq, notifs, push
}

func newEnqueuerHarnessWithZones(t *testing.T, tier profiles.SubscriptionTier, zones *fakeZones) (*Enqueuer, *fakeNotifications, *fakePush, *fakeZones) {
	t.Helper()
	notifs := newFakeNotifications()
	profile := profileWithTier(t, "auth0|alice", tier)
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|alice": profile}}
	devs := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{
		"auth0|alice": {{Token: "tok-1"}},
	}}
	st := &fakeState{unread: 2}
	push := &fakePush{}
	if zones == nil {
		zones = &fakeZones{}
	}
	enq := NewEnqueuer(notifs, zones, profs, devs, st, push,
		func() string { return "n-fixed" },
		func() time.Time { return time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC) },
		testLogger(t))
	return enq, notifs, push, zones
}

func TestEnqueuer_EnqueueForApplication_FansOutToContainingZones(t *testing.T) {
	t.Parallel()
	// Two zones (different owners) contain the point; the enqueuer must fan out
	// to both users.
	z1 := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	z2 := testZoneAt(t, "zone-2", "auth0|bob", time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{z1, z2}}
	notifs := newFakeNotifications()
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{
		"auth0|alice": profileWithTier(t, "auth0|alice", profiles.TierPro),
		"auth0|bob":   profileWithTier(t, "auth0|bob", profiles.TierPro),
	}}
	devs := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{}}
	fz := zones
	enq := NewEnqueuer(notifs, zones, profs, devs, &fakeState{}, &fakePush{},
		func() string { return "n-fixed" },
		func() time.Time { return time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC) },
		testLogger(t))
	app := testApplication(t, 99, "24/0001", 51.5, -0.1, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))

	if err := enq.EnqueueForApplication(context.Background(), app); err != nil {
		t.Fatalf("EnqueueForApplication: %v", err)
	}
	if fz.lastLat != 51.5 || fz.lastLng != -0.1 {
		t.Errorf("zone lookup point: got lat=%v lng=%v", fz.lastLat, fz.lastLng)
	}
	if len(notifs.created) != 2 {
		t.Errorf("expected one record per containing zone owner, got %d", len(notifs.created))
	}
}

func TestEnqueuer_EnqueueForApplication_OneUserTwoZonesDedupsToOneRecord(t *testing.T) {
	t.Parallel()
	// A single user with two containing zones must still get ONE NewApplication
	// notification — dedup is per (user, application, authority), not per zone.
	z1 := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	z2 := testZoneAt(t, "zone-2", "auth0|alice", time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{z1, z2}}
	enq, notifs, _, _ := newEnqueuerHarnessWithZones(t, profiles.TierPro, zones)
	app := testApplication(t, 99, "24/0001", 51.5, -0.1, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))

	if err := enq.EnqueueForApplication(context.Background(), app); err != nil {
		t.Fatalf("EnqueueForApplication: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Errorf("one user with two zones must dedup to one record, got %d", len(notifs.created))
	}
}

func TestEnqueuer_EnqueueForApplication_SkipsZonesCreatedAfterLastDifferent(t *testing.T) {
	t.Parallel()
	lastDifferent := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	// One zone created before the change (eligible), one created after (skip).
	before := testZoneAt(t, "zone-old", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	after := testZoneAt(t, "zone-new", "auth0|alice", time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{before, after}}
	enq, notifs, _, _ := newEnqueuerHarnessWithZones(t, profiles.TierPro, zones)
	app := testApplication(t, 99, "24/0001", 51.5, -0.1, lastDifferent)

	if err := enq.EnqueueForApplication(context.Background(), app); err != nil {
		t.Fatalf("EnqueueForApplication: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Fatalf("only zones created on/before LastDifferent should fan out, got %d", len(notifs.created))
	}
	if notifs.created[0].WatchZoneID == nil || *notifs.created[0].WatchZoneID != "zone-old" {
		t.Errorf("wrong zone fanned out: %+v", notifs.created[0].WatchZoneID)
	}
}

func TestEnqueuer_EnqueueForApplication_NoCoordsSkipsLookup(t *testing.T) {
	t.Parallel()
	zones := &fakeZones{zones: []watchzones.WatchZone{
		testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)),
	}}
	enq, notifs, _, fz := newEnqueuerHarnessWithZones(t, profiles.TierPro, zones)
	app := applications.PlanningApplication{
		Name: "24/0001", UID: "24/0001", AreaID: 99,
		LastDifferent: time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC),
	}

	if err := enq.EnqueueForApplication(context.Background(), app); err != nil {
		t.Fatalf("EnqueueForApplication: %v", err)
	}
	if fz.lastLat != 0 || fz.lastLng != 0 {
		t.Errorf("zone lookup must be skipped when the application has no coordinates")
	}
	if len(notifs.created) != 0 {
		t.Errorf("no coordinates means no fan-out, got %d", len(notifs.created))
	}
}

func TestEnqueuer_PaidTier_CreatesRecordAndPushes(t *testing.T) {
	t.Parallel()
	enq, notifs, push := newEnqueuerHarness(t, profiles.TierPro)
	app := testApplication(t, 99, "24/0001", 51.5, -0.1, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))

	if err := enq.Enqueue(context.Background(), app, zone); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Fatalf("expected 1 created record, got %d", len(notifs.created))
	}
	rec := notifs.created[0]
	if rec.UserID != "auth0|alice" || rec.ApplicationUID != "24/0001" || rec.AuthorityID != 99 {
		t.Errorf("record fields: %+v", rec)
	}
	if rec.WatchZoneID == nil || *rec.WatchZoneID != "zone-1" {
		t.Errorf("record should carry the matching zone id: %+v", rec.WatchZoneID)
	}
	if rec.EventType != notifications.EventNewApplication {
		t.Errorf("event type: got %q, want NewApplication", rec.EventType)
	}
	if push.calls != 1 {
		t.Errorf("paid tier should push exactly once, got %d", push.calls)
	}
	if len(push.tokens) != 1 || push.tokens[0] != "tok-1" {
		t.Errorf("push tokens: got %v", push.tokens)
	}
}

func TestEnqueuer_FreeTier_CreatesRecordNoPush(t *testing.T) {
	t.Parallel()
	enq, notifs, push := newEnqueuerHarness(t, profiles.TierFree)
	app := testApplication(t, 99, "24/0001", 51.5, -0.1, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))

	if err := enq.Enqueue(context.Background(), app, zone); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Fatalf("free tier must still create the digest record, got %d", len(notifs.created))
	}
	if push.calls != 0 {
		t.Errorf("free tier must NOT push, got %d calls", push.calls)
	}
}

func TestEnqueuer_Dedup_SkipsWhenAlreadyNotified(t *testing.T) {
	t.Parallel()
	enq, notifs, push := newEnqueuerHarness(t, profiles.TierPro)
	app := testApplication(t, 99, "24/0001", 51.5, -0.1, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))

	if err := enq.Enqueue(context.Background(), app, zone); err != nil {
		t.Fatalf("first Enqueue: %v", err)
	}
	if err := enq.Enqueue(context.Background(), app, zone); err != nil {
		t.Fatalf("second Enqueue: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Errorf("re-enqueue must not double-create, got %d records", len(notifs.created))
	}
	if push.calls != 1 {
		t.Errorf("re-enqueue must not double-push, got %d calls", push.calls)
	}
}

func TestEnqueuer_UnknownProfile_NoRecord(t *testing.T) {
	t.Parallel()
	enq, notifs, push := newEnqueuerHarness(t, profiles.TierPro)
	app := testApplication(t, 99, "24/0001", 51.5, -0.1, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
	zone := testZoneAt(t, "zone-1", "auth0|stranger", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))

	if err := enq.Enqueue(context.Background(), app, zone); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(notifs.created) != 0 {
		t.Errorf("unknown profile must not create a record, got %d", len(notifs.created))
	}
	if push.calls != 0 {
		t.Errorf("unknown profile must not push, got %d", push.calls)
	}
}

func TestEnqueuer_PaidTier_NoDevicesStillWritesRecord(t *testing.T) {
	t.Parallel()
	notifs := newFakeNotifications()
	profile := profileWithTier(t, "auth0|alice", profiles.TierPersonal)
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|alice": profile}}
	devs := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{}}
	st := &fakeState{}
	push := &fakePush{}
	enq := NewEnqueuer(notifs, &fakeZones{}, profs, devs, st, push,
		func() string { return "n-1" },
		func() time.Time { return time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC) },
		testLogger(t))
	app := testApplication(t, 99, "24/0001", 51.5, -0.1, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))

	if err := enq.Enqueue(context.Background(), app, zone); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Errorf("record must be written even when there are no devices to push to, got %d", len(notifs.created))
	}
	if push.calls != 0 {
		t.Errorf("no devices means no push call, got %d", push.calls)
	}
}

func TestEnqueuer_PrunesInvalidTokens(t *testing.T) {
	t.Parallel()
	notifs := newFakeNotifications()
	profile := profileWithTier(t, "auth0|alice", profiles.TierPro)
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|alice": profile}}
	devs := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{
		"auth0|alice": {{Token: "good"}, {Token: "stale"}},
	}}
	st := &fakeState{}
	push := &fakePush{invalid: []string{"stale"}}
	enq := NewEnqueuer(notifs, &fakeZones{}, profs, devs, st, push,
		func() string { return "n-1" },
		func() time.Time { return time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC) },
		testLogger(t))
	app := testApplication(t, 99, "24/0001", 51.5, -0.1, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))

	if err := enq.Enqueue(context.Background(), app, zone); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(devs.deleted) != 1 || devs.deleted[0] != "stale" {
		t.Errorf("invalid token should be pruned: got %v", devs.deleted)
	}
}
