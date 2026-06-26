package main

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
)

// discardLogger is a no-op logger so tests stay quiet.
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// rawDeviceDoc builds a minimal valid DeviceRegistrations-container document body.
func rawDeviceDoc(userID, token string) []byte {
	return []byte(`{"id":"` + token + `","userId":"` + userID + `","token":"` + token + `",` +
		`"platform":"Ios","registeredAt":"2026-06-26T12:00:00+00:00","ttl":15552000}`)
}

// fakeDevicePager yields successive pages of raw document bodies, or a fixed error.
type fakeDevicePager struct {
	pages [][][]byte
	idx   int
	err   error
}

func (p *fakeDevicePager) More() bool { return p.idx < len(p.pages) }

func (p *fakeDevicePager) NextPage(_ context.Context) ([][]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	page := p.pages[p.idx]
	p.idx++
	return page, nil
}

// fakeDeviceUpserter records every saved registration and can fail by token.
type fakeDeviceUpserter struct {
	saved  []devicetokens.DeviceRegistration
	failOn map[string]error
}

func (u *fakeDeviceUpserter) Save(_ context.Context, reg devicetokens.DeviceRegistration) error {
	if err := u.failOn[reg.Token]; err != nil {
		return err
	}
	u.saved = append(u.saved, reg)
	return nil
}

func TestDeviceBackfill_SavesAllRecordsAcrossPages(t *testing.T) {
	t.Parallel()
	pager := &fakeDevicePager{pages: [][][]byte{
		{rawDeviceDoc("auth0|u1", "tok-a"), rawDeviceDoc("auth0|u2", "tok-b")},
		{rawDeviceDoc("auth0|u3", "tok-c")},
	}}
	store := &fakeDeviceUpserter{}

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

func TestDeviceBackfill_ContinuesPastDecodeError(t *testing.T) {
	t.Parallel()
	pager := &fakeDevicePager{pages: [][][]byte{
		{rawDeviceDoc("auth0|u1", "tok-a"), []byte("not json"), rawDeviceDoc("auth0|u2", "tok-b")},
	}}
	store := &fakeDeviceUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 3 || got.upserted != 2 || got.errors != 1 {
		t.Errorf("summary: got %+v, want read=3 upserted=2 errors=1", got)
	}
}

func TestDeviceBackfill_ContinuesPastSaveError(t *testing.T) {
	t.Parallel()
	pager := &fakeDevicePager{pages: [][][]byte{
		{rawDeviceDoc("auth0|u1", "tok-a"), rawDeviceDoc("auth0|u2", "tok-b"), rawDeviceDoc("auth0|u3", "tok-c")},
	}}
	store := &fakeDeviceUpserter{failOn: map[string]error{"tok-b": errors.New("boom")}}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 3 || got.upserted != 2 || got.errors != 1 {
		t.Errorf("summary: got %+v, want read=3 upserted=2 errors=1", got)
	}
}

func TestDeviceBackfill_RespectsLimitAcrossPages(t *testing.T) {
	t.Parallel()
	pager := &fakeDevicePager{pages: [][][]byte{
		{rawDeviceDoc("auth0|u1", "tok-a")},
		{rawDeviceDoc("auth0|u2", "tok-b")},
		{rawDeviceDoc("auth0|u3", "tok-c")},
	}}
	store := &fakeDeviceUpserter{}

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

func TestDeviceBackfill_PagerErrorReturnsError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("cosmos read failed")
	pager := &fakeDevicePager{pages: [][][]byte{{rawDeviceDoc("auth0|u1", "tok-a")}}, err: sentinel}
	store := &fakeDeviceUpserter{}

	_, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, sentinel) {
		t.Fatalf("backfill: got err %v, want wrapped %v", err, sentinel)
	}
}

func TestDeviceBackfill_ContextCancelAborts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pager := &fakeDevicePager{pages: [][][]byte{{rawDeviceDoc("auth0|u1", "tok-a")}}}
	store := &fakeDeviceUpserter{}

	_, err := backfill(ctx, pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("backfill: got err %v, want context.Canceled", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("store: saved %d records, want 0 (cancelled before any save)", len(store.saved))
	}
}
