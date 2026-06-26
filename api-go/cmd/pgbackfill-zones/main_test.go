package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// discardLogger is a no-op logger so tests stay quiet.
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// rawZoneDoc builds a minimal valid WatchZones-container document body for id,
// with a London centre so the decode produces a valid domain zone.
func rawZoneDoc(id string) []byte {
	return []byte(`{"id":"` + id + `","userId":"auth0|u-` + id + `","name":"Zone ` + id + `",` +
		`"latitude":51.5074,"longitude":-0.1278,"radiusMetres":1000,"authorityId":100,` +
		`"location":{"type":"Point","coordinates":[-0.1278,51.5074]},` +
		`"createdAt":"2026-06-26T12:00:00+00:00","pushEnabled":true,"emailInstantEnabled":true}`)
}

// fakeZonePager yields successive pages of raw document bodies, or a fixed error.
type fakeZonePager struct {
	pages [][][]byte
	idx   int
	err   error
}

func (p *fakeZonePager) More() bool { return p.idx < len(p.pages) }

func (p *fakeZonePager) NextPage(_ context.Context) ([][]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	page := p.pages[p.idx]
	p.idx++
	return page, nil
}

// fakeZoneUpserter records every saved zone and can fail by zone id.
type fakeZoneUpserter struct {
	saved  []watchzones.WatchZone
	failOn map[string]error
}

func (u *fakeZoneUpserter) Save(_ context.Context, z watchzones.WatchZone) error {
	if err := u.failOn[z.ID]; err != nil {
		return err
	}
	u.saved = append(u.saved, z)
	return nil
}

func TestBackfill_SavesAllRecordsAcrossPages(t *testing.T) {
	t.Parallel()
	pager := &fakeZonePager{pages: [][][]byte{
		{rawZoneDoc("a"), rawZoneDoc("b")},
		{rawZoneDoc("c")},
	}}
	store := &fakeZoneUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 3 || got.upserted != 3 || got.errors != 0 {
		t.Errorf("summary: got %+v, want read=3 upserted=3 errors=0", got)
	}
	if len(store.saved) != 3 {
		t.Errorf("store: saved %d records, want 3", len(store.saved))
	}
}

func TestBackfill_ContinuesPastDecodeError(t *testing.T) {
	t.Parallel()
	pager := &fakeZonePager{pages: [][][]byte{
		{rawZoneDoc("a"), []byte("not json"), rawZoneDoc("b")},
	}}
	store := &fakeZoneUpserter{}

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
	pager := &fakeZonePager{pages: [][][]byte{
		{rawZoneDoc("a"), rawZoneDoc("b"), rawZoneDoc("c")},
	}}
	store := &fakeZoneUpserter{failOn: map[string]error{"b": errors.New("boom")}}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 3 || got.upserted != 2 || got.errors != 1 {
		t.Errorf("summary: got %+v, want read=3 upserted=2 errors=1", got)
	}
}

func TestBackfill_RespectsLimitAcrossPages(t *testing.T) {
	t.Parallel()
	pager := &fakeZonePager{pages: [][][]byte{
		{rawZoneDoc("a")},
		{rawZoneDoc("b")},
		{rawZoneDoc("c")},
	}}
	store := &fakeZoneUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{limit: 2}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 2 || got.upserted != 2 {
		t.Errorf("summary: got %+v, want read=2 upserted=2", got)
	}
	if pager.idx > 2 {
		t.Errorf("pager consumed %d pages, want it to stop at the limit (<=2)", pager.idx)
	}
}

func TestBackfill_PagerErrorReturnsError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("cosmos read failed")
	pager := &fakeZonePager{pages: [][][]byte{{rawZoneDoc("a")}}, err: sentinel}
	store := &fakeZoneUpserter{}

	_, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, sentinel) {
		t.Fatalf("backfill: got err %v, want wrapped %v", err, sentinel)
	}
}

func TestBackfill_ContextCancelAborts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pager := &fakeZonePager{pages: [][][]byte{{rawZoneDoc("a")}}}
	store := &fakeZoneUpserter{}

	_, err := backfill(ctx, pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("backfill: got err %v, want context.Canceled", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("store: saved %d records, want 0 (cancelled before any save)", len(store.saved))
	}
}
