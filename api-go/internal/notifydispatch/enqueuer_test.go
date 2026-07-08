package notifydispatch

import (
	"context"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
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

// queuedPush is one Add call a fakePushQueue recorded.
type queuedPush struct {
	userID       string
	notification notifications.DigestNotification
}

// fakePushQueue stands in for the coalescer from the dispatchers' side: it just
// records what it was asked to queue, so the enqueuer/decision tests assert
// "queued when eligible" without exercising the coalescer's own send/flush
// behaviour (covered separately in coalescer_test.go).
type fakePushQueue struct {
	queued []queuedPush
}

func (f *fakePushQueue) Add(userID string, n notifications.DigestNotification) {
	f.queued = append(f.queued, queuedPush{userID: userID, notification: n})
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

func newEnqueuerHarness(t *testing.T, tier profiles.SubscriptionTier) (*Enqueuer, *fakeNotifications, *fakePushQueue) {
	t.Helper()
	enq, notifs, queue, _ := newEnqueuerHarnessWithZones(t, tier, nil)
	return enq, notifs, queue
}

func newEnqueuerHarnessWithZones(t *testing.T, tier profiles.SubscriptionTier, zones *fakeZones) (*Enqueuer, *fakeNotifications, *fakePushQueue, *fakeZones) {
	t.Helper()
	notifs := newFakeNotifications()
	profile := profileWithTier(t, "auth0|alice", tier)
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|alice": profile}}
	queue := &fakePushQueue{}
	if zones == nil {
		zones = &fakeZones{}
	}
	enq := NewEnqueuer(notifs, zones, profs, queue,
		func() string { return "n-fixed" },
		func() time.Time { return time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC) },
		testLogger(t))
	return enq, notifs, queue, zones
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
	fz := zones
	enq := NewEnqueuer(notifs, zones, profs, &fakePushQueue{},
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

func TestEnqueuer_PaidTier_CreatesRecordAndQueuesPush(t *testing.T) {
	t.Parallel()
	enq, notifs, queue := newEnqueuerHarness(t, profiles.TierPro)
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
	if !rec.PushSent {
		t.Error("PushSent should be true once the notification is queued for a push-eligible user")
	}
	if len(queue.queued) != 1 {
		t.Fatalf("paid tier should queue exactly one push, got %d", len(queue.queued))
	}
	if queue.queued[0].userID != "auth0|alice" {
		t.Errorf("queued push user: got %q, want auth0|alice", queue.queued[0].userID)
	}
}

func TestEnqueuer_FreeTier_CreatesRecordNoPush(t *testing.T) {
	t.Parallel()
	enq, notifs, queue := newEnqueuerHarness(t, profiles.TierFree)
	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))

	if err := enq.Enqueue(context.Background(), app, zone); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Fatalf("free tier must still create the digest record, got %d", len(notifs.created))
	}
	if notifs.created[0].PushSent {
		t.Error("free tier record must not read PushSent=true")
	}
	if len(queue.queued) != 0 {
		t.Errorf("free tier must NOT queue a push, got %d", len(queue.queued))
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
	queue := &fakePushQueue{}
	enq := NewEnqueuer(notifs, &fakeZones{}, profs, queue,
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
	if len(queue.queued) != 0 {
		t.Errorf("lapsed paid tier must NOT queue a push, got %d", len(queue.queued))
	}
}

func TestEnqueuer_Dedup_SkipsWhenAlreadyNotified(t *testing.T) {
	t.Parallel()
	enq, notifs, queue := newEnqueuerHarness(t, profiles.TierPro)
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
	if len(queue.queued) != 1 {
		t.Errorf("re-enqueue must not double-queue a push, got %d", len(queue.queued))
	}
}

func TestEnqueuer_FreeTierOverQuota_PausesNewerZones(t *testing.T) {
	t.Parallel()
	// Free tier (limit 1) with 3 zones ranked oldest-first by (CreatedAt, ID):
	// only rank 1 (zone-1, the oldest) is active; rank 2 (zone-2) is paused.
	z1 := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	z2 := testZoneAt(t, "zone-2", "auth0|alice", time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC))
	z3 := testZoneAt(t, "zone-3", "auth0|alice", time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC))
	allZones := []watchzones.WatchZone{z1, z2, z3}

	t.Run("rank 2 creates no record", func(t *testing.T) {
		t.Parallel()
		zones := &fakeZones{zones: allZones}
		enq, notifs, queue, _ := newEnqueuerHarnessWithZones(t, profiles.TierFree, zones)
		app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))

		if err := enq.Enqueue(context.Background(), app, z2); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		if len(notifs.created) != 0 {
			t.Errorf("paused (over-quota) zone must create NO record, got %d", len(notifs.created))
		}
		if len(queue.queued) != 0 {
			t.Errorf("paused zone must not queue a push, got %d", len(queue.queued))
		}
		if !zones.getByUserIDCalled {
			t.Error("bounded (non-unlimited) tier must call GetByUserID to rank the zone")
		}
	})

	t.Run("rank 1 creates a record", func(t *testing.T) {
		t.Parallel()
		zones := &fakeZones{zones: allZones}
		enq, notifs, _, _ := newEnqueuerHarnessWithZones(t, profiles.TierFree, zones)
		app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))

		if err := enq.Enqueue(context.Background(), app, z1); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		if len(notifs.created) != 1 {
			t.Errorf("the active (rank 1) zone must still create a record, got %d", len(notifs.created))
		}
	})
}

func TestEnqueuer_PersonalTierAtQuota_AllZonesActive(t *testing.T) {
	t.Parallel()
	// Personal tier (limit 3) with exactly 3 zones: nobody is over quota, so all
	// three must still create records.
	z1 := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	z2 := testZoneAt(t, "zone-2", "auth0|alice", time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC))
	z3 := testZoneAt(t, "zone-3", "auth0|alice", time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC))
	allZones := []watchzones.WatchZone{z1, z2, z3}

	for _, z := range allZones {
		z := z
		t.Run(z.ID, func(t *testing.T) {
			t.Parallel()
			zones := &fakeZones{zones: allZones}
			enq, notifs, _, _ := newEnqueuerHarnessWithZones(t, profiles.TierPersonal, zones)
			app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))

			if err := enq.Enqueue(context.Background(), app, z); err != nil {
				t.Fatalf("Enqueue: %v", err)
			}
			if len(notifs.created) != 1 {
				t.Errorf("zone %s: at-quota Personal tier must still create a record, got %d", z.ID, len(notifs.created))
			}
		})
	}
}

func TestEnqueuer_ProTierOverTenZones_AllActiveAndSkipsRankingQuery(t *testing.T) {
	t.Parallel()
	// Pro tier is unlimited (math.MaxInt32): every one of the 10 zones must
	// create a record, and the fast path must never call GetByUserID.
	var allZones []watchzones.WatchZone
	for i := range 10 {
		allZones = append(allZones, testZoneAt(t, strconv.Itoa(i), "auth0|alice",
			time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i)))
	}

	for _, z := range allZones {
		z := z
		t.Run(z.ID, func(t *testing.T) {
			t.Parallel()
			zones := &fakeZones{zones: allZones}
			enq, notifs, _, _ := newEnqueuerHarnessWithZones(t, profiles.TierPro, zones)
			app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))

			if err := enq.Enqueue(context.Background(), app, z); err != nil {
				t.Fatalf("Enqueue: %v", err)
			}
			if len(notifs.created) != 1 {
				t.Errorf("zone %s: Pro (unlimited) tier must still create a record, got %d", z.ID, len(notifs.created))
			}
			if zones.getByUserIDCalled {
				t.Error("unlimited tier must skip the GetByUserID ranking query entirely")
			}
		})
	}
}

func TestEnqueuer_LapsedPaidTier_RankedAgainstFreeLimit(t *testing.T) {
	t.Parallel()
	// Stored Personal (limit 3), but expired with no grace: EffectiveTier
	// collapses to Free (limit 1), so rank 2 must be paused even though the
	// stored tier would have allowed it.
	notifs := newFakeNotifications()
	profile := profileWithTier(t, "auth0|alice", profiles.TierPersonal)
	past := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) // before the harness clock
	profile.SubscriptionExpiry = &past
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|alice": profile}}
	queue := &fakePushQueue{}

	z1 := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	z2 := testZoneAt(t, "zone-2", "auth0|alice", time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{z1, z2}}
	enq := NewEnqueuer(notifs, zones, profs, queue,
		func() string { return "n-1" },
		func() time.Time { return time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC) },
		testLogger(t))
	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))

	if err := enq.Enqueue(context.Background(), app, z2); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(notifs.created) != 0 {
		t.Errorf("a lapsed paid tier must be ranked against the Free limit, got %d records for rank-2 zone", len(notifs.created))
	}
}

func TestEnqueuer_UnknownProfile_NoRecord(t *testing.T) {
	t.Parallel()
	enq, notifs, queue := newEnqueuerHarness(t, profiles.TierPro)
	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
	zone := testZoneAt(t, "zone-1", "auth0|stranger", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))

	if err := enq.Enqueue(context.Background(), app, zone); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if len(notifs.created) != 0 {
		t.Errorf("unknown profile must not create a record, got %d", len(notifs.created))
	}
	if len(queue.queued) != 0 {
		t.Errorf("unknown profile must not queue a push, got %d", len(queue.queued))
	}
}
