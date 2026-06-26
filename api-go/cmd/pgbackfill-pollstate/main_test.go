package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/polling"
)

// discardLogger returns a no-op logger so test output stays clean.
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// rawPollStateDoc builds a minimal valid PollState-container document body for
// the given authorityId, using the dotNetRoundTrip layout the Cosmos store emits.
func rawPollStateDoc(authorityID int) []byte {
	ts := "2026-06-14T12:00:00.0000000+00:00"
	return []byte(fmt.Sprintf(
		`{"id":"poll-state-%d","authorityId":%d,"lastPollTime":"%s","highWaterMark":"%s"}`,
		authorityID, authorityID, ts, ts,
	))
}

// fakePollStatePager yields successive pages of raw document bodies.
type fakePollStatePager struct {
	pages [][][]byte
	idx   int
	err   error
}

func (p *fakePollStatePager) More() bool { return p.idx < len(p.pages) }

func (p *fakePollStatePager) NextPage(_ context.Context) ([][]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	page := p.pages[p.idx]
	p.idx++
	return page, nil
}

// fakePollStateUpserter records every call to Save and can fail on demand.
type fakePollStateUpserter struct {
	saved  []savedCall
	failOn map[int]error
}

type savedCall struct {
	authorityID int
	lastPoll    time.Time
}

func (u *fakePollStateUpserter) Save(_ context.Context, authorityID int, lastPollTime, _ time.Time, _ *polling.PollCursor) error {
	if err := u.failOn[authorityID]; err != nil {
		return err
	}
	u.saved = append(u.saved, savedCall{authorityID: authorityID, lastPoll: lastPollTime})
	return nil
}

func TestBackfillPollState_SavesAllRecordsAcrossPages(t *testing.T) {
	t.Parallel()
	pager := &fakePollStatePager{pages: [][][]byte{
		{rawPollStateDoc(1), rawPollStateDoc(2)},
		{rawPollStateDoc(3)},
	}}
	store := &fakePollStateUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 3 || got.upserted != 3 || got.errors != 0 {
		t.Errorf("summary: got %+v, want read=3 upserted=3 errors=0", got)
	}
	if len(store.saved) != 3 {
		t.Errorf("saved %d records, want 3", len(store.saved))
	}
}

func TestBackfillPollState_ContinuesPastDecodeError(t *testing.T) {
	t.Parallel()
	pager := &fakePollStatePager{pages: [][][]byte{
		{rawPollStateDoc(1), []byte("not json"), rawPollStateDoc(2)},
	}}
	store := &fakePollStateUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 3 || got.upserted != 2 || got.errors != 1 {
		t.Errorf("summary: got %+v, want read=3 upserted=2 errors=1", got)
	}
}

func TestBackfillPollState_ContinuesPastSaveError(t *testing.T) {
	t.Parallel()
	pager := &fakePollStatePager{pages: [][][]byte{
		{rawPollStateDoc(1), rawPollStateDoc(2), rawPollStateDoc(3)},
	}}
	store := &fakePollStateUpserter{failOn: map[int]error{2: errors.New("deadlock")}}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 3 || got.upserted != 2 || got.errors != 1 {
		t.Errorf("summary: got %+v, want read=3 upserted=2 errors=1", got)
	}
}

func TestBackfillPollState_RespectsLimit(t *testing.T) {
	t.Parallel()
	pager := &fakePollStatePager{pages: [][][]byte{
		{rawPollStateDoc(1)},
		{rawPollStateDoc(2)},
		{rawPollStateDoc(3)},
	}}
	store := &fakePollStateUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{limit: 2}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 2 || got.upserted != 2 {
		t.Errorf("summary: got %+v, want read=2 upserted=2", got)
	}
	if pager.idx > 2 {
		t.Errorf("pager consumed %d pages, want it to stop at limit (<=2)", pager.idx)
	}
}

func TestBackfillPollState_PagerErrorReturnsError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("cosmos unreachable")
	pager := &fakePollStatePager{pages: [][][]byte{{rawPollStateDoc(1)}}, err: sentinel}
	store := &fakePollStateUpserter{}

	_, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, sentinel) {
		t.Fatalf("backfill: got err %v, want wrapped %v", err, sentinel)
	}
}

func TestBackfillPollState_ContextCancelAborts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pager := &fakePollStatePager{pages: [][][]byte{{rawPollStateDoc(1)}}}
	store := &fakePollStateUpserter{}

	_, err := backfill(ctx, pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("backfill: got err %v, want context.Canceled", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("saved %d records, want 0 (cancelled before any save)", len(store.saved))
	}
}
