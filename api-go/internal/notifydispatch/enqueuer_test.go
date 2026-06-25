package notifydispatch

import (
	"context"
	"encoding/json"
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
	return slog.New(slog.DiscardHandler)
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

func testApplication(t *testing.T, lastDifferent time.Time) applications.PlanningApplication {
	t.Helper()
	la, lo := 51.5, -0.1
	state := "Pending"
	return applications.PlanningApplication{
		Name:          "24/0001",
		UID:           "24/0001",
		AreaName:      "Kingston",
		AreaID:        99,
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
	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))

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

func TestEnqueuer_EnqueueForApplication_MatchesCrossBorderNeighbourAuthorityZone(t *testing.T) {
	t.Parallel()
	// Boundary-agnostic matching (tc-b179 / tc-w11n): a zone pinned to authority 449
	// (Adur & Worthing) must receive a NewApplication for an in-circle application
	// tagged authority 246 (Arun) on the other side of the border. The store no
	// longer scopes the lookup by authority, so the fake returns the 449 zone for
	// the 246 app. The CreatedAt.After(LastDifferent) skip rule is UNCHANGED: a
	// second 449 zone created after the application last changed is still skipped.
	lastDifferent := time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)
	eligible, err := watchzones.NewWatchZone("zone-449", "auth0|alice", "Border", 50.81, -0.42, 2000, 449,
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), true, true)
	if err != nil {
		t.Fatalf("NewWatchZone eligible: %v", err)
	}
	tooNew, err := watchzones.NewWatchZone("zone-449-new", "auth0|alice", "Border New", 50.81, -0.42, 2000, 449,
		time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), true, true)
	if err != nil {
		t.Fatalf("NewWatchZone tooNew: %v", err)
	}
	zones := &fakeZones{zones: []watchzones.WatchZone{eligible, tooNew}}
	enq, notifs, _, fz := newEnqueuerHarnessWithZones(t, profiles.TierPro, zones)

	// The application is tagged the NEIGHBOUR authority (246), not the zone's (449).
	la, lo := 50.815, -0.41
	state := "Pending"
	app := applications.PlanningApplication{
		Name: "AR/0007", UID: "AR/0007", AreaName: "Arun", AreaID: 246,
		AppState: &state, Latitude: &la, Longitude: &lo, LastDifferent: lastDifferent,
	}

	if err := enq.EnqueueForApplication(context.Background(), app); err != nil {
		t.Fatalf("EnqueueForApplication: %v", err)
	}
	if fz.lastLat != la || fz.lastLng != lo {
		t.Errorf("zone lookup point: got lat=%v lng=%v, want %v %v", fz.lastLat, fz.lastLng, la, lo)
	}
	if len(notifs.created) != 1 {
		t.Fatalf("border-spanning app must enqueue for the neighbour-authority zone exactly once, got %d", len(notifs.created))
	}
	if notifs.created[0].WatchZoneID == nil || *notifs.created[0].WatchZoneID != "zone-449" {
		t.Errorf("wrong zone fanned out (the post-LastDifferent zone must be skipped): %+v", notifs.created[0].WatchZoneID)
	}
}

func TestEnqueuer_EnqueueForApplication_NonBorderZoneMatchesUnchanged(t *testing.T) {
	t.Parallel()
	// Regression: a zone that does NOT cross a boundary (same authority as the app)
	// still matches exactly as before the authority prune was removed — one record.
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) // authority 99
	zones := &fakeZones{zones: []watchzones.WatchZone{zone}}
	enq, notifs, _, _ := newEnqueuerHarnessWithZones(t, profiles.TierPro, zones)
	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)) // authority 99

	if err := enq.EnqueueForApplication(context.Background(), app); err != nil {
		t.Fatalf("EnqueueForApplication: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Errorf("same-authority zone must still match exactly once, got %d", len(notifs.created))
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
	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))

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
	app := testApplication(t, lastDifferent)

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
	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
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
	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
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

func TestEnqueuer_ExpiredPaidTier_CreatesRecordNoPush(t *testing.T) {
	t.Parallel()
	// A paid tier whose subscription has lapsed (past expiry, no grace) is treated
	// as Free: the digest record is still written, but no instant push fires.
	notifs := newFakeNotifications()
	profile := profileWithTier(t, "auth0|alice", profiles.TierPro)
	past := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) // before the harness clock
	profile.SubscriptionExpiry = &past
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|alice": profile}}
	devs := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{
		"auth0|alice": {{Token: "tok-1"}},
	}}
	push := &fakePush{}
	enq := NewEnqueuer(notifs, &fakeZones{}, profs, devs, &fakeState{}, push,
		func() string { return "n-1" },
		func() time.Time { return time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC) },
		testLogger(t))
	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))

	if err := enq.Enqueue(context.Background(), app, zone); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Fatalf("lapsed paid tier must still create the digest record, got %d", len(notifs.created))
	}
	if push.calls != 0 {
		t.Errorf("lapsed paid tier must NOT push, got %d calls", push.calls)
	}
}

func TestEnqueuer_Dedup_SkipsWhenAlreadyNotified(t *testing.T) {
	t.Parallel()
	enq, notifs, push := newEnqueuerHarness(t, profiles.TierPro)
	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
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
	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
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
	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
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
	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))

	if err := enq.Enqueue(context.Background(), app, zone); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(devs.deleted) != 1 || devs.deleted[0] != "stale" {
		t.Errorf("invalid token should be pruned: got %v", devs.deleted)
	}
}
