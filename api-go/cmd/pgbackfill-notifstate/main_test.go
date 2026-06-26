package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/notificationstate"
)

// discardLogger is a no-op logger so tests stay quiet.
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// rawStateDoc builds a minimal valid NotificationState-container document body
// for userId. The JSON shape mirrors the stateDocument wire format (camelCase
// keys, DotNetTime lastReadAt), which is what the Cosmos pager yields.
func rawStateDoc(userID string) []byte {
	return []byte(`{"id":"` + userID + `","userId":"` + userID + `",` +
		`"lastReadAt":"2026-06-26T12:00:00+00:00","version":1}`)
}

// fakeStatePager yields successive pages of raw document bodies, or a fixed error.
type fakeStatePager struct {
	pages [][][]byte
	idx   int
	err   error
}

func (p *fakeStatePager) More() bool { return p.idx < len(p.pages) }

func (p *fakeStatePager) NextPage(_ context.Context) ([][]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	page := p.pages[p.idx]
	p.idx++
	return page, nil
}

// fakeStateSaver records every saved state and can fail by user id.
type fakeStateSaver struct {
	saved  []notificationstate.State
	failOn map[string]error
}

func (u *fakeStateSaver) Save(_ context.Context, st notificationstate.State) error {
	if err := u.failOn[st.UserID]; err != nil {
		return err
	}
	u.saved = append(u.saved, st)
	return nil
}

func TestBackfill_SavesAllRecordsAcrossPages(t *testing.T) {
	t.Parallel()
	pager := &fakeStatePager{pages: [][][]byte{
		{rawStateDoc("auth0|u-a"), rawStateDoc("auth0|u-b")},
		{rawStateDoc("auth0|u-c")},
	}}
	store := &fakeStateSaver{}

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
	pager := &fakeStatePager{pages: [][][]byte{
		{rawStateDoc("auth0|u-a"), []byte("not json"), rawStateDoc("auth0|u-b")},
	}}
	store := &fakeStateSaver{}

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
	pager := &fakeStatePager{pages: [][][]byte{
		{rawStateDoc("auth0|u-a"), rawStateDoc("auth0|u-b"), rawStateDoc("auth0|u-c")},
	}}
	store := &fakeStateSaver{failOn: map[string]error{"auth0|u-b": errors.New("boom")}}

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
	pager := &fakeStatePager{pages: [][][]byte{
		{rawStateDoc("auth0|u-a")},
		{rawStateDoc("auth0|u-b")},
		{rawStateDoc("auth0|u-c")},
	}}
	store := &fakeStateSaver{}

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
	pager := &fakeStatePager{pages: [][][]byte{{rawStateDoc("auth0|u-a")}}, err: sentinel}
	store := &fakeStateSaver{}

	_, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, sentinel) {
		t.Fatalf("backfill: got err %v, want wrapped %v", err, sentinel)
	}
}

func TestBackfill_ContextCancelAborts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pager := &fakeStatePager{pages: [][][]byte{{rawStateDoc("auth0|u-a")}}}
	store := &fakeStateSaver{}

	_, err := backfill(ctx, pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("backfill: got err %v, want context.Canceled", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("store: saved %d records, want 0 (cancelled before any save)", len(store.saved))
	}
}
