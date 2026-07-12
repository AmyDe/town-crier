package polling

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- tests: Ingester exercised directly, independent of PollPlanItHandler ---

func TestIngester_UpsertsNewApplication(t *testing.T) {
	t.Parallel()
	apps := newFakeApps()
	ing := NewIngester(apps, nil, nil)
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	app := testApp("24/0001", 99, ld)

	if err := ing.Ingest(context.Background(), app); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(apps.upserts) != 1 {
		t.Errorf("upserts: got %d, want 1", len(apps.upserts))
	}
}

func TestIngester_SkipsUpsertWhenBusinessFieldsUnchanged(t *testing.T) {
	t.Parallel()
	apps := newFakeApps()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	app := testApp("24/0001", 99, ld)
	existing := app
	existing.LastDifferent = ld.Add(-time.Hour)
	apps.existing[app.UID] = existing
	ing := NewIngester(apps, nil, nil)

	if err := ing.Ingest(context.Background(), app); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(apps.upserts) != 0 {
		t.Errorf("unchanged business fields must skip upsert, got %d upserts, want 0", len(apps.upserts))
	}
}

func TestIngester_DispatchesDecisionOnTransitionAndEnqueues(t *testing.T) {
	t.Parallel()
	apps := newFakeApps()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	apps.existing["24/0001/FUL"] = decisionApp("Undecided", ld.Add(-time.Hour))
	disp := &fakeDecisionDispatcher{}
	enq := &fakeEnqueuer{}
	ing := NewIngester(apps, disp, enq)

	if err := ing.Ingest(context.Background(), decisionApp("Permitted", ld)); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if disp.count() != 1 {
		t.Errorf("decision dispatch count: got %d, want 1", disp.count())
	}
	if enq.count() != 1 {
		t.Errorf("enqueue count: got %d, want 1", enq.count())
	}
}

func TestIngester_NoDecisionDispatchWhenAlreadyDecided(t *testing.T) {
	t.Parallel()
	apps := newFakeApps()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	apps.existing["24/0001/FUL"] = decisionApp("Permitted", ld.Add(-time.Hour))
	disp := &fakeDecisionDispatcher{}
	enq := &fakeEnqueuer{}
	ing := NewIngester(apps, disp, enq)

	if err := ing.Ingest(context.Background(), decisionApp("Conditions", ld)); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if disp.count() != 0 {
		t.Errorf("decision->decision change is not a new transition, got %d dispatches, want 0", disp.count())
	}
	if enq.count() != 1 {
		t.Errorf("the changed application should still be enqueued, got %d, want 1", enq.count())
	}
}

func TestIngester_NilCollaboratorsSkipFanOutGracefully(t *testing.T) {
	t.Parallel()
	apps := newFakeApps()
	ing := NewIngester(apps, nil, nil)
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)

	if err := ing.Ingest(context.Background(), decisionApp("Permitted", ld)); err != nil {
		t.Fatalf("Ingest with nil fan-out collaborators must not error: %v", err)
	}
	if len(apps.upserts) != 1 {
		t.Errorf("upsert must still happen with nil fan-out collaborators, got %d, want 1", len(apps.upserts))
	}
}

func TestIngester_PropagatesGetByUIDError(t *testing.T) {
	t.Parallel()
	apps := newFakeApps()
	wantErr := errors.New("store unavailable")
	apps.getErr = wantErr
	ing := NewIngester(apps, nil, nil)

	err := ing.Ingest(context.Background(), testApp("24/0001", 99, time.Now()))
	if !errors.Is(err, wantErr) {
		t.Errorf("Ingest error: got %v, want %v", err, wantErr)
	}
}

// --- tests: the three-bucket change classification (GH#935) ---

func TestIngester_SilentOnlyChange_UpsertsWithoutFanOut(t *testing.T) {
	t.Parallel()
	apps := newFakeApps()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	existing := testApp("24/0001", 99, ld)
	existing.OtherFields = map[string]any{"comment_url": "https://old"}
	apps.existing[existing.UID] = existing

	incoming := existing
	incoming.OtherFields = map[string]any{"comment_url": "https://new"}

	disp := &fakeDecisionDispatcher{}
	enq := &fakeEnqueuer{}
	ing := NewIngester(apps, disp, enq)

	if err := ing.Ingest(context.Background(), incoming); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(apps.upserts) != 1 {
		t.Errorf("silent-only change must still upsert, got %d, want 1", len(apps.upserts))
	}
	if enq.count() != 0 {
		t.Errorf("silent-only change must NOT enqueue, got %d, want 0", enq.count())
	}
	if disp.count() != 0 {
		t.Errorf("silent-only change must NOT dispatch a decision, got %d, want 0", disp.count())
	}
}

func TestIngester_BookkeepingOnlyChange_SkipsUpsertEntirely(t *testing.T) {
	t.Parallel()
	apps := newFakeApps()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	existing := testApp("24/0001", 99, ld)
	existing.LastChanged = timePtr(ld)
	existing.LastScraped = timePtr(ld)
	apps.existing[existing.UID] = existing

	incoming := existing
	incoming.LastDifferent = ld.Add(48 * time.Hour) // PlanIt re-index bump
	incoming.LastChanged = timePtr(ld.Add(time.Hour))
	incoming.LastScraped = timePtr(ld.Add(2 * time.Hour))

	disp := &fakeDecisionDispatcher{}
	enq := &fakeEnqueuer{}
	ing := NewIngester(apps, disp, enq)

	if err := ing.Ingest(context.Background(), incoming); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(apps.upserts) != 0 {
		t.Errorf("bookkeeping-only change must skip the upsert entirely, got %d, want 0", len(apps.upserts))
	}
	if enq.count() != 0 || disp.count() != 0 {
		t.Errorf("bookkeeping-only change must not fan out: enqueue=%d dispatch=%d", enq.count(), disp.count())
	}
}

func TestIngester_SilentOnlyChangeOnAlreadyDecidedApp_DoesNotRedispatchDecision(t *testing.T) {
	t.Parallel()
	apps := newFakeApps()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	existing := decisionApp("Permitted", ld)
	existing.OtherFields = map[string]any{"comment_url": "https://old"}
	apps.existing[existing.UID] = existing

	incoming := existing
	incoming.OtherFields = map[string]any{"comment_url": "https://new"}

	disp := &fakeDecisionDispatcher{}
	enq := &fakeEnqueuer{}
	ing := NewIngester(apps, disp, enq)

	if err := ing.Ingest(context.Background(), incoming); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if disp.count() != 0 {
		t.Errorf("silent-only change on an already-decided app must not re-dispatch, got %d, want 0", disp.count())
	}
	if len(apps.upserts) != 1 {
		t.Errorf("silent-only change must still upsert, got %d, want 1", len(apps.upserts))
	}
}

func timePtr(t time.Time) *time.Time { return &t }

func TestIngester_PropagatesUpsertError(t *testing.T) {
	t.Parallel()
	apps := newFakeApps()
	wantErr := errors.New("upsert failed")
	apps.upsertErr = wantErr
	ing := NewIngester(apps, nil, nil)

	err := ing.Ingest(context.Background(), testApp("24/0001", 99, time.Now()))
	if !errors.Is(err, wantErr) {
		t.Errorf("Ingest error: got %v, want %v", err, wantErr)
	}
}
