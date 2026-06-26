package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// discardLogger is a no-op logger so tests stay quiet.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// rawDoc builds a minimal valid Applications-container document body for name,
// with a London point so the decode produces coordinates.
func rawDoc(name string) []byte {
	return []byte(`{"planitName":"` + name + `","authorityCode":"100","areaId":100,` +
		`"uid":"u-` + name + `","areaName":"Testshire","address":"1 Test Street",` +
		`"description":"d","location":{"type":"Point","coordinates":[-0.1278,51.5074]},` +
		`"lastDifferent":"2026-06-26T12:00:00+00:00"}`)
}

// fakePager yields successive pages of raw document bodies, or a fixed error.
type fakePager struct {
	pages [][][]byte
	idx   int
	err   error
}

func (p *fakePager) More() bool { return p.idx < len(p.pages) }

func (p *fakePager) NextPage(_ context.Context) ([][]byte, error) {
	if p.err != nil {
		return nil, p.err
	}
	page := p.pages[p.idx]
	p.idx++
	return page, nil
}

// fakeUpserter records every upserted application and can fail by app name.
type fakeUpserter struct {
	upserted []applications.PlanningApplication
	failOn   map[string]error
}

func (u *fakeUpserter) Upsert(_ context.Context, a applications.PlanningApplication) error {
	if err := u.failOn[a.Name]; err != nil {
		return err
	}
	u.upserted = append(u.upserted, a)
	return nil
}

func TestBackfill_UpsertsAllRecordsAcrossPages(t *testing.T) {
	t.Parallel()
	pager := &fakePager{pages: [][][]byte{
		{rawDoc("a"), rawDoc("b")},
		{rawDoc("c")},
	}}
	store := &fakeUpserter{}

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
	pager := &fakePager{pages: [][][]byte{
		{rawDoc("a"), []byte("not json"), rawDoc("b")},
	}}
	store := &fakeUpserter{}

	got, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got.read != 3 || got.upserted != 2 || got.errors != 1 {
		t.Errorf("summary: got %+v, want read=3 upserted=2 errors=1", got)
	}
}

func TestBackfill_ContinuesPastUpsertError(t *testing.T) {
	t.Parallel()
	pager := &fakePager{pages: [][][]byte{
		{rawDoc("a"), rawDoc("b"), rawDoc("c")},
	}}
	store := &fakeUpserter{failOn: map[string]error{"b": errors.New("boom")}}

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
	pager := &fakePager{pages: [][][]byte{
		{rawDoc("a")},
		{rawDoc("b")},
		{rawDoc("c")},
	}}
	store := &fakeUpserter{}

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
	pager := &fakePager{pages: [][][]byte{{rawDoc("a")}}, err: sentinel}
	store := &fakeUpserter{}

	_, err := backfill(context.Background(), pager, store, backfillOptions{}, discardLogger())
	if !errors.Is(err, sentinel) {
		t.Fatalf("backfill: got err %v, want wrapped %v", err, sentinel)
	}
}
