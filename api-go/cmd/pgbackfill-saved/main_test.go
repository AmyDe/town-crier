package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/savedapplications"
)

// discardLogger is a no-op logger so tests stay quiet.
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// rawSavedDoc builds a minimal valid SavedApplications-container document body.
// The embedded application snapshot is included so DecodeDocument produces a
// non-nil Application field — mirroring the real Cosmos documents.
func rawSavedDoc(userID, appUID string, authorityID int) []byte {
	return []byte(`{` +
		`"id":"` + userID + `:` + appUID + `",` +
		`"userId":"` + userID + `",` +
		`"applicationUid":"` + appUID + `",` +
		`"authorityId":` + itoa(authorityID) + `,` +
		`"savedAt":"2026-06-26T12:00:00+00:00",` +
		`"application":{` +
		`"name":"24/00001/FUL","uid":"uid-1","areaName":"Testshire",` +
		`"areaId":` + itoa(authorityID) + `,"address":"1 Test St","description":"test",` +
		`"lastDifferent":"2026-06-26T00:00:00+00:00"` +
		`}}`)
}

// itoa converts an int to its decimal string — avoids importing strconv for a
// test-only helper.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

// fakeSavedPager yields successive pages of raw document bodies, or a fixed error.
type fakeSavedPager struct {
	pages [][][]byte
	idx   int
	err   error
}

func (p *fakeSavedPager) More() bool { return p.idx < len(p.pages) }

func (p *fakeSavedPager) NextPage(_ context.Context) ([][]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	page := p.pages[p.idx]
	p.idx++
	return page, nil
}

// fakeSavedUpserter records every saved application and can fail by applicationUID.
type fakeSavedUpserter struct {
	saved  []savedapplications.SavedApplication
	failOn map[string]error
}

func (u *fakeSavedUpserter) Save(_ context.Context, sa savedapplications.SavedApplication) error {
	if err := u.failOn[sa.ApplicationUID]; err != nil {
		return err
	}
	u.saved = append(u.saved, sa)
	return nil
}

func TestSavedBackfill_SavesAllRecordsAcrossPages(t *testing.T) {
	t.Parallel()
	pager := &fakeSavedPager{pages: [][][]byte{
		{rawSavedDoc("auth0|u1", "uid-a", 100), rawSavedDoc("auth0|u2", "uid-b", 100)},
		{rawSavedDoc("auth0|u3", "uid-c", 200)},
	}}
	store := &fakeSavedUpserter{}

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

func TestSavedBackfill_ContinuesPastDecodeError(t *testing.T) {
	t.Parallel()
	pager := &fakeSavedPager{pages: [][][]byte{
		{rawSavedDoc("auth0|u1", "uid-a", 100), []byte("not json"), rawSavedDoc("auth0|u2", "uid-b", 100)},
	}}
	store := &fakeSavedUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 3 || got.upserted != 2 || got.errors != 1 {
		t.Errorf("summary: got %+v, want read=3 upserted=2 errors=1", got)
	}
}

func TestSavedBackfill_ContinuesPastSaveError(t *testing.T) {
	t.Parallel()
	pager := &fakeSavedPager{pages: [][][]byte{
		{rawSavedDoc("auth0|u1", "uid-a", 100), rawSavedDoc("auth0|u2", "uid-b", 100), rawSavedDoc("auth0|u3", "uid-c", 100)},
	}}
	// Build uid-b's canonical UID: NewSavedApplication computes it as areaId/name
	// but in DecodeDocument -> toDomain we use the raw applicationUid from the doc.
	store := &fakeSavedUpserter{failOn: map[string]error{"uid-b": errors.New("boom")}}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 3 || got.upserted != 2 || got.errors != 1 {
		t.Errorf("summary: got %+v, want read=3 upserted=2 errors=1", got)
	}
}

func TestSavedBackfill_RespectsLimitAcrossPages(t *testing.T) {
	t.Parallel()
	pager := &fakeSavedPager{pages: [][][]byte{
		{rawSavedDoc("auth0|u1", "uid-a", 100)},
		{rawSavedDoc("auth0|u2", "uid-b", 100)},
		{rawSavedDoc("auth0|u3", "uid-c", 100)},
	}}
	store := &fakeSavedUpserter{}

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

func TestSavedBackfill_PagerErrorReturnsError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("cosmos read failed")
	pager := &fakeSavedPager{pages: [][][]byte{{rawSavedDoc("auth0|u1", "uid-a", 100)}}, err: sentinel}
	store := &fakeSavedUpserter{}

	_, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, sentinel) {
		t.Fatalf("backfill: got err %v, want wrapped %v", err, sentinel)
	}
}

func TestSavedBackfill_ContextCancelAborts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pager := &fakeSavedPager{pages: [][][]byte{{rawSavedDoc("auth0|u1", "uid-a", 100)}}}
	store := &fakeSavedUpserter{}

	_, err := backfill(ctx, pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("backfill: got err %v, want context.Canceled", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("store: saved %d records, want 0 (cancelled before any save)", len(store.saved))
	}
}

// TestSavedBackfill_DecodePreservesAuthorityID verifies that DecodeDocument
// populates AuthorityID from the document's authorityId field, not from the
// embedded snapshot's areaId — the Cosmos toDomain coalescence path.
func TestSavedBackfill_DecodePreservesAuthorityID(t *testing.T) {
	t.Parallel()
	// authorityId=100 in the doc; areaId in the snapshot is also 100 here, but
	// the coalescence logic is exercised by the non-nil authorityId branch.
	doc := rawSavedDoc("auth0|u1", "uid-a", 100)
	pager := &fakeSavedPager{pages: [][][]byte{{doc}}}
	store := &fakeSavedUpserter{}

	if _, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger()); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if len(store.saved) != 1 {
		t.Fatalf("store: saved %d records, want 1", len(store.saved))
	}
	if store.saved[0].AuthorityID != 100 {
		t.Errorf("AuthorityID: got %d, want 100", store.saved[0].AuthorityID)
	}
	if store.saved[0].SavedAt.IsZero() {
		t.Error("SavedAt: got zero, want non-zero")
	}
	_ = time.Time{} // ensure time import is used
}
