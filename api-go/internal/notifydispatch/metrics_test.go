package notifydispatch

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// fakeNotifMetrics records the notification-created calls a dispatcher makes. It
// satisfies the notifydispatch consumer-side notificationMetricsRecorder.
type fakeNotifMetrics struct {
	eventTypes []string
	sources    []string
}

func (f *fakeNotifMetrics) NotificationCreated(_ context.Context, eventType, sources string) {
	f.eventTypes = append(f.eventTypes, eventType)
	f.sources = append(f.sources, sources)
}

func TestEnqueuer_RecordsNotificationCreated(t *testing.T) {
	t.Parallel()
	z := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{z}}
	enq, _, _, _ := newEnqueuerHarnessWithZones(t, profiles.TierPro, zones)
	rec := &fakeNotifMetrics{}
	enq.WithMetrics(rec)

	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
	if err := enq.EnqueueForApplication(context.Background(), app); err != nil {
		t.Fatalf("EnqueueForApplication: %v", err)
	}

	if len(rec.eventTypes) != 1 || rec.eventTypes[0] != "NewApplication" {
		t.Errorf("NotificationCreated event types = %v, want [NewApplication]", rec.eventTypes)
	}
	if len(rec.sources) != 1 || rec.sources[0] != "Zone" {
		t.Errorf("NotificationCreated sources = %v, want [Zone]", rec.sources)
	}
}

func TestEnqueuer_DoesNotRecordOnDedup(t *testing.T) {
	t.Parallel()
	z := testZoneAt(t, "zone-1", "auth0|alice", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	zones := &fakeZones{zones: []watchzones.WatchZone{z}}
	enq, notifs, _, _ := newEnqueuerHarnessWithZones(t, profiles.TierPro, zones)
	rec := &fakeNotifMetrics{}
	enq.WithMetrics(rec)

	app := testApplication(t, time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC))
	if err := enq.EnqueueForApplication(context.Background(), app); err != nil {
		t.Fatalf("first EnqueueForApplication: %v", err)
	}
	// Second fan-out of the same application is a dedup no-op: no new record, no
	// new metric.
	if err := enq.EnqueueForApplication(context.Background(), app); err != nil {
		t.Fatalf("second EnqueueForApplication: %v", err)
	}
	if len(notifs.created) != 1 {
		t.Fatalf("expected one record after dedup, got %d", len(notifs.created))
	}
	if len(rec.eventTypes) != 1 {
		t.Errorf("NotificationCreated must fire once, got %d", len(rec.eventTypes))
	}
}
