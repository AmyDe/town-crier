package notifydispatch

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// recordingContainer is a hand-written stand-in for the Cosmos container the
// real notifications.DigestStore writes through. It captures upserts keyed by
// partition and replays them on the single-partition query the digest reader
// (ByUserSince) issues — enough to prove a poll-path-written record round-trips
// through the digest read path unchanged.
type recordingContainer struct {
	stored map[string][][]byte // partitionKey -> raw document bodies
}

func newRecordingContainer() *recordingContainer {
	return &recordingContainer{stored: map[string][][]byte{}}
}

func (c *recordingContainer) UpsertItem(_ context.Context, partitionKey string, item []byte) error {
	cp := make([]byte, len(item))
	copy(cp, item)
	c.stored[partitionKey] = append(c.stored[partitionKey], cp)
	return nil
}

func (c *recordingContainer) QueryItems(_ context.Context, partitionKey, _ string, _ map[string]any) ([][]byte, error) {
	return c.stored[partitionKey], nil
}

func (c *recordingContainer) QueryItemsCrossPartition(_ context.Context, _ string, _ map[string]any) ([][]byte, error) {
	return nil, nil
}

// TestEnqueuer_CreatedRecordIsDigestReadable is the load-bearing cross-package
// proof for bead tc-uc2p: a notification the poll-path enqueuer creates lands in
// the Notifications container in the exact shape the digest worker reads. We wire
// the real *notifications.DigestStore over a fake container, enqueue, then read
// the same bytes back through the digest's ByUserSince — the path the weekly
// digest uses — and assert the application surfaces with its fields intact.
func TestEnqueuer_CreatedRecordIsDigestReadable(t *testing.T) {
	t.Parallel()
	container := newRecordingContainer()
	store := notifications.NewDigestStore(container)

	profile := profileWithTier(t, "auth0|alice", profiles.TierFree) // Free: record only, no push
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|alice": profile}}
	devs := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{}}
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{zone}}

	enq := NewEnqueuer(store, zones, profs, devs, &fakeState{}, &fakePush{},
		func() string { return "n-roundtrip" },
		func() time.Time { return time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC) },
		testLogger(t))

	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))

	if err := enq.EnqueueForApplication(context.Background(), app); err != nil {
		t.Fatalf("EnqueueForApplication: %v", err)
	}

	// Read it back exactly as the weekly digest does.
	since := time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC)
	got, err := store.ByUserSince(context.Background(), "auth0|alice", since)
	if err != nil {
		t.Fatalf("ByUserSince: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("digest must read back exactly one record, got %d", len(got))
	}
	n := got[0]
	if n.ID != "n-roundtrip" {
		t.Errorf("id round-trip: got %q", n.ID)
	}
	if n.UserID != "auth0|alice" || n.ApplicationUID != "24/0001" || n.AuthorityID != 99 {
		t.Errorf("core fields lost in round-trip: %+v", n)
	}
	if n.ApplicationAddress != "10 High St" || n.ApplicationDescription != "Loft conversion" {
		t.Errorf("display fields lost in round-trip: %+v", n)
	}
	if n.WatchZoneID == nil || *n.WatchZoneID != "zone-1" {
		t.Errorf("zone attribution lost: %+v", n.WatchZoneID)
	}
	if n.EventType != notifications.EventNewApplication {
		t.Errorf("event type lost: got %q", n.EventType)
	}
	if n.EmailSent {
		t.Error("a freshly enqueued record must be unsent so the hourly/weekly digest picks it up")
	}
}
