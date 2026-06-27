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

// TestEnqueuer_CreatedRecordIsDigestReadable is the cross-package proof for bead
// tc-uc2p: a notification the poll-path enqueuer creates lands in the
// Notifications store in the exact shape the digest worker reads. The single
// Postgres store serves both the enqueuer's Create and the digest's ByUserSince,
// so capturing the created DigestNotification and asserting its fields proves the
// poll-path-written record is digest-readable with every field intact.
func TestEnqueuer_CreatedRecordIsDigestReadable(t *testing.T) {
	t.Parallel()
	notifs := newFakeNotifications()

	profile := profileWithTier(t, "auth0|alice", profiles.TierFree) // Free: record only, no push
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|alice": profile}}
	devs := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{}}
	zone := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{zone}}

	enq := NewEnqueuer(notifs, zones, profs, devs, &fakeState{}, &fakePush{},
		func() string { return "n-roundtrip" },
		func() time.Time { return time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC) },
		testLogger(t))

	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))

	if err := enq.EnqueueForApplication(context.Background(), app); err != nil {
		t.Fatalf("EnqueueForApplication: %v", err)
	}

	if len(notifs.created) != 1 {
		t.Fatalf("enqueuer must create exactly one record, got %d", len(notifs.created))
	}
	n := notifs.created[0]
	if n.ID != "n-roundtrip" {
		t.Errorf("id: got %q", n.ID)
	}
	if n.UserID != "auth0|alice" || n.ApplicationUID != "24/0001" || n.AuthorityID != 99 {
		t.Errorf("core fields lost: %+v", n)
	}
	if n.ApplicationAddress != "10 High St" || n.ApplicationDescription != "Loft conversion" {
		t.Errorf("display fields lost: %+v", n)
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
