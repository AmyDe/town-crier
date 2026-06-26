package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/subscriptions"
)

// discardLogger is a no-op logger so tests stay quiet.
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// rawNotifDoc builds a minimal valid AppleNotifications-container document
// body for the given UUID.
func rawNotifDoc(uuid string) []byte {
	return []byte(`{"id":"` + uuid + `","processedAt":"2026-01-15T09:00:00+00:00"}`)
}

// fakeNotifPager yields successive pages of raw document bodies, or a fixed error.
type fakeNotifPager struct {
	pages [][][]byte
	idx   int
	err   error
}

func (p *fakeNotifPager) More() bool { return p.idx < len(p.pages) }

func (p *fakeNotifPager) NextPage(_ context.Context) ([][]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	page := p.pages[p.idx]
	p.idx++
	return page, nil
}

// fakeNotifUpserter records every upserted notification and can fail on a
// specific UUID.
type fakeNotifUpserter struct {
	upserted []subscriptions.ProcessedNotification
	failOn   map[string]error
}

func (u *fakeNotifUpserter) UpsertProcessed(_ context.Context, uuid string, processedAt time.Time) error {
	if err := u.failOn[uuid]; err != nil {
		return err
	}
	u.upserted = append(u.upserted, subscriptions.ProcessedNotification{UUID: uuid, ProcessedAt: processedAt})
	return nil
}

func TestBackfill_SavesAllRecordsAcrossPages(t *testing.T) {
	t.Parallel()
	pager := &fakeNotifPager{pages: [][][]byte{
		{rawNotifDoc("uuid-a"), rawNotifDoc("uuid-b")},
		{rawNotifDoc("uuid-c")},
	}}
	store := &fakeNotifUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 3 || got.upserted != 3 || got.errors != 0 {
		t.Errorf("summary: got %+v, want read=3 upserted=3 errors=0", got)
	}
	if len(store.upserted) != 3 {
		t.Errorf("store: upserted %d records, want 3", len(store.upserted))
	}
}

func TestBackfill_ContinuesPastDecodeError(t *testing.T) {
	t.Parallel()
	pager := &fakeNotifPager{pages: [][][]byte{
		{rawNotifDoc("uuid-a"), []byte("not json"), rawNotifDoc("uuid-b")},
	}}
	store := &fakeNotifUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 3 || got.upserted != 2 || got.errors != 1 {
		t.Errorf("summary: got %+v, want read=3 upserted=2 errors=1", got)
	}
}

func TestBackfill_ContinuesPastSaveError(t *testing.T) {
	t.Parallel()
	pager := &fakeNotifPager{pages: [][][]byte{
		{rawNotifDoc("uuid-a"), rawNotifDoc("uuid-b"), rawNotifDoc("uuid-c")},
	}}
	store := &fakeNotifUpserter{failOn: map[string]error{"uuid-b": errors.New("boom")}}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 3 || got.upserted != 2 || got.errors != 1 {
		t.Errorf("summary: got %+v, want read=3 upserted=2 errors=1", got)
	}
}

func TestBackfill_RespectsLimit(t *testing.T) {
	t.Parallel()
	pager := &fakeNotifPager{pages: [][][]byte{
		{rawNotifDoc("uuid-a")},
		{rawNotifDoc("uuid-b")},
		{rawNotifDoc("uuid-c")},
	}}
	store := &fakeNotifUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{limit: 2}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 2 || got.upserted != 2 {
		t.Errorf("summary: got %+v, want read=2 upserted=2", got)
	}
	if pager.idx > 2 {
		t.Errorf("pager consumed %d pages; should stop at limit (<=2)", pager.idx)
	}
}

func TestBackfill_PagerErrorReturnsError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("cosmos read failed")
	pager := &fakeNotifPager{pages: [][][]byte{{rawNotifDoc("uuid-a")}}, err: sentinel}
	store := &fakeNotifUpserter{}

	_, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, sentinel) {
		t.Fatalf("got err %v, want wrapped sentinel", err)
	}
}

func TestBackfill_ContextCancelAborts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pager := &fakeNotifPager{pages: [][][]byte{{rawNotifDoc("uuid-a")}}}
	store := &fakeNotifUpserter{}

	_, err := backfill(ctx, pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got err %v, want context.Canceled", err)
	}
	if len(store.upserted) != 0 {
		t.Errorf("store: upserted %d records, want 0 (cancelled before any save)", len(store.upserted))
	}
}

func TestBackfill_PreservesProcessedAt(t *testing.T) {
	t.Parallel()
	pager := &fakeNotifPager{pages: [][][]byte{
		{rawNotifDoc("uuid-ts")},
	}}
	store := &fakeNotifUpserter{}

	_, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if len(store.upserted) != 1 {
		t.Fatalf("expected 1 upserted, got %d", len(store.upserted))
	}
	want := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	if !store.upserted[0].ProcessedAt.Equal(want) {
		t.Errorf("ProcessedAt = %v, want %v", store.upserted[0].ProcessedAt, want)
	}
}
