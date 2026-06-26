package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/offercodes"
)

// discardLogger is a no-op logger so tests stay quiet.
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// rawCodeDoc builds a minimal valid OfferCodes-container document body for the
// given code. The tier and duration are fixed so the decode produces a valid
// domain OfferCode.
func rawCodeDoc(code string) []byte {
	return []byte(`{"id":"` + code + `","code":"` + code + `","tier":"Pro","durationDays":30,` +
		`"createdAt":"2026-06-26T12:00:00+00:00","redeemed":false}`)
}

// fakeCodePager yields successive pages of raw document bodies, or a fixed
// error.
type fakeCodePager struct {
	pages [][][]byte
	idx   int
	err   error
}

func (p *fakeCodePager) More() bool { return p.idx < len(p.pages) }

func (p *fakeCodePager) NextPage(_ context.Context) ([][]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	page := p.pages[p.idx]
	p.idx++
	return page, nil
}

// fakeCodeUpserter records every saved offer code and can fail by code value.
type fakeCodeUpserter struct {
	saved  []offercodes.OfferCode
	failOn map[string]error
}

func (u *fakeCodeUpserter) Save(_ context.Context, c offercodes.OfferCode) error {
	if err := u.failOn[c.Code]; err != nil {
		return err
	}
	u.saved = append(u.saved, c)
	return nil
}

func TestBackfill_SavesAllRecordsAcrossPages(t *testing.T) {
	t.Parallel()
	pager := &fakeCodePager{pages: [][][]byte{
		{rawCodeDoc("AAAAAAAAAAAA"), rawCodeDoc("BBBBBBBBBBBB")},
		{rawCodeDoc("CCCCCCCCCCCC")},
	}}
	store := &fakeCodeUpserter{}

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
	pager := &fakeCodePager{pages: [][][]byte{
		{rawCodeDoc("AAAAAAAAAAAA"), []byte("not json"), rawCodeDoc("BBBBBBBBBBBB")},
	}}
	store := &fakeCodeUpserter{}

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
	pager := &fakeCodePager{pages: [][][]byte{
		{rawCodeDoc("AAAAAAAAAAAA"), rawCodeDoc("BBBBBBBBBBBB"), rawCodeDoc("CCCCCCCCCCCC")},
	}}
	store := &fakeCodeUpserter{failOn: map[string]error{"BBBBBBBBBBBB": errors.New("boom")}}

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
	pager := &fakeCodePager{pages: [][][]byte{
		{rawCodeDoc("AAAAAAAAAAAA")},
		{rawCodeDoc("BBBBBBBBBBBB")},
		{rawCodeDoc("CCCCCCCCCCCC")},
	}}
	store := &fakeCodeUpserter{}

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
	pager := &fakeCodePager{pages: [][][]byte{{rawCodeDoc("AAAAAAAAAAAA")}}, err: sentinel}
	store := &fakeCodeUpserter{}

	_, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, sentinel) {
		t.Fatalf("backfill: got err %v, want wrapped %v", err, sentinel)
	}
}

func TestBackfill_ContextCancelAborts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pager := &fakeCodePager{pages: [][][]byte{{rawCodeDoc("AAAAAAAAAAAA")}}}
	store := &fakeCodeUpserter{}

	_, err := backfill(ctx, pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("backfill: got err %v, want context.Canceled", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("store: saved %d records, want 0 (cancelled before any save)", len(store.saved))
	}
}
