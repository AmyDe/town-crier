package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// discardLogger is a no-op logger so tests stay quiet.
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// rawProfileDoc builds a minimal valid Users-container document body for the
// given userId, ready for profiles.DecodeDocument.
func rawProfileDoc(userID string) []byte {
	return []byte(`{` +
		`"id":"` + userID + `",` +
		`"userId":"` + userID + `",` +
		`"email":"` + userID + `@example.com",` +
		`"pushEnabled":true,` +
		`"digestDay":1,` +
		`"emailDigestEnabled":true,` +
		`"savedDecisionPush":true,` +
		`"savedDecisionEmail":true,` +
		`"zonePreferences":{},` +
		`"tier":"Free",` +
		`"lastActiveAt":"2026-06-26T12:00:00Z",` +
		`"lastActiveAtEpoch":1719403200000` +
		`}`)
}

// fakeProfilePager yields successive pages of raw document bodies, or a fixed
// error.
type fakeProfilePager struct {
	pages [][][]byte
	idx   int
	err   error
}

func (p *fakeProfilePager) More() bool { return p.idx < len(p.pages) }

func (p *fakeProfilePager) NextPage(_ context.Context) ([][]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	page := p.pages[p.idx]
	p.idx++
	return page, nil
}

// fakeProfileUpserter records every saved profile and can fail by userID.
type fakeProfileUpserter struct {
	saved  []*profiles.UserProfile
	failOn map[string]error
}

func (u *fakeProfileUpserter) Save(_ context.Context, p *profiles.UserProfile) error {
	if err := u.failOn[p.UserID]; err != nil {
		return err
	}
	u.saved = append(u.saved, p)
	return nil
}

func TestBackfill_SavesAllRecordsAcrossPages(t *testing.T) {
	t.Parallel()
	pager := &fakeProfilePager{pages: [][][]byte{
		{rawProfileDoc("auth0|u1"), rawProfileDoc("auth0|u2")},
		{rawProfileDoc("auth0|u3")},
	}}
	store := &fakeProfileUpserter{}

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
	pager := &fakeProfilePager{pages: [][][]byte{
		{rawProfileDoc("auth0|u1"), []byte("not json"), rawProfileDoc("auth0|u2")},
	}}
	store := &fakeProfileUpserter{}

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
	pager := &fakeProfilePager{pages: [][][]byte{
		{rawProfileDoc("auth0|u1"), rawProfileDoc("auth0|u2"), rawProfileDoc("auth0|u3")},
	}}
	store := &fakeProfileUpserter{failOn: map[string]error{"auth0|u2": errors.New("boom")}}

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
	pager := &fakeProfilePager{pages: [][][]byte{
		{rawProfileDoc("auth0|u1")},
		{rawProfileDoc("auth0|u2")},
		{rawProfileDoc("auth0|u3")},
	}}
	store := &fakeProfileUpserter{}

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
	pager := &fakeProfilePager{pages: [][][]byte{{rawProfileDoc("auth0|u1")}}, err: sentinel}
	store := &fakeProfileUpserter{}

	_, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, sentinel) {
		t.Fatalf("backfill: got err %v, want wrapped %v", err, sentinel)
	}
}

func TestBackfill_ContextCancelAborts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pager := &fakeProfilePager{pages: [][][]byte{{rawProfileDoc("auth0|u1")}}}
	store := &fakeProfileUpserter{}

	_, err := backfill(ctx, pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("backfill: got err %v, want context.Canceled", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("store: saved %d records, want 0 (cancelled before any save)", len(store.saved))
	}
}

// TestBackfill_UnknownTierCounted verifies that a document with an unrecognised
// tier ("Platinum") is rejected by DecodeDocument and counted as an error,
// not silently defaulted to Free.
func TestBackfill_UnknownTierCounted(t *testing.T) {
	t.Parallel()
	bad := []byte(`{
		"id":"auth0|ux","userId":"auth0|ux",
		"pushEnabled":false,"digestDay":0,"zonePreferences":{},
		"tier":"Platinum",
		"lastActiveAt":"2026-06-26T12:00:00Z","lastActiveAtEpoch":1719403200000
	}`)
	pager := &fakeProfilePager{pages: [][][]byte{{bad}}}
	store := &fakeProfileUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.errors != 1 || got.upserted != 0 {
		t.Errorf("summary: got %+v, want errors=1 upserted=0", got)
	}
}

// TestBackfill_CoalesceDefaultsPreserved verifies that a legacy document with
// absent nullable-bool fields (emailDigestEnabled absent) decodes without error
// (coalesceTrue path) and is saved correctly.
func TestBackfill_CoalesceDefaultsPreserved(t *testing.T) {
	t.Parallel()
	legacy := []byte(`{
		"id":"auth0|leg","userId":"auth0|leg",
		"pushEnabled":false,"digestDay":0,"zonePreferences":{},
		"tier":"Free",
		"lastActiveAt":"2026-06-26T12:00:00Z","lastActiveAtEpoch":1719403200000
	}`)
	pager := &fakeProfilePager{pages: [][][]byte{{legacy}}}
	store := &fakeProfileUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.upserted != 1 || got.errors != 0 {
		t.Errorf("summary: got %+v, want upserted=1 errors=0", got)
	}
	if len(store.saved) != 1 {
		t.Fatalf("saved %d profiles, want 1", len(store.saved))
	}
	if !store.saved[0].Preferences.EmailDigestEnabled {
		t.Error("EmailDigestEnabled should coalesce to true for legacy document")
	}
}

// TestBackfill_LogProgressEveryN verifies the logEvery cadence does not panic
// and does not suppress records.
func TestBackfill_LogProgressEveryN(t *testing.T) {
	t.Parallel()

	// 5 records, log every 2.
	docs := make([][]byte, 0, 5)
	for i := range 5 {
		docs = append(docs, rawProfileDoc("auth0|p"+string(rune('0'+i))))
	}
	pager := &fakeProfilePager{pages: [][][]byte{docs}}
	store := &fakeProfileUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{logEvery: 2}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 5 || got.upserted != 5 {
		t.Errorf("summary: got %+v, want read=5 upserted=5", got)
	}
}

// Ensure the fake satisfies the interface at compile time.
var _ profileUpserter = (*fakeProfileUpserter)(nil)
