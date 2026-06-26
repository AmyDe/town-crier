//go:build integration

package main

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// rawDocWithState builds a valid Applications-container document body for name
// with a given app_state, so the re-run can assert an in-place field update.
func rawDocWithState(name, state string) []byte {
	return []byte(`{"planitName":"` + name + `","authorityCode":"100","areaId":100,` +
		`"uid":"u-` + name + `","areaName":"Testshire","address":"1 Test Street",` +
		`"description":"d","appState":"` + state + `",` +
		`"location":{"type":"Point","coordinates":[-0.1278,51.5074]},` +
		`"lastDifferent":"2026-06-26T12:00:00+00:00"}`)
}

// TestBackfill_IdempotentReRun proves the ON CONFLICT (authority_code,
// planit_name) DO UPDATE path against a real PostGIS: backfilling the same record
// twice leaves exactly one row, and the second run updates fields in place.
//
// A test that calls pgtest.New must NOT call t.Parallel (it holds a session-level
// advisory lock released in t.Cleanup).
func TestBackfill_IdempotentReRun(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications")

	store := applications.NewPostgresStore(pool)

	first := &fakePager{pages: [][][]byte{{rawDocWithState("24/0001", "Pending")}}}
	s1, err := backfill(ctx, first, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	if s1.read != 1 || s1.upserted != 1 || s1.errors != 0 {
		t.Fatalf("first summary: got %+v, want read=1 upserted=1 errors=0", s1)
	}
	assertRowCount(t, pool, 1)

	second := &fakePager{pages: [][][]byte{{rawDocWithState("24/0001", "Permitted")}}}
	s2, err := backfill(ctx, second, store, backfillOptions{}, discardLogger())
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if s2.upserted != 1 {
		t.Fatalf("second summary: got %+v, want upserted=1", s2)
	}

	assertRowCount(t, pool, 1)

	got, ok, err := store.GetByAuthorityAndName(ctx, "100", "24/0001")
	if err != nil || !ok {
		t.Fatalf("read back 24/0001: ok=%v err=%v", ok, err)
	}
	if got.AppState == nil || *got.AppState != "Permitted" {
		t.Errorf("app_state after re-run: got %v, want Permitted", got.AppState)
	}
	if got.Longitude == nil || got.Latitude == nil {
		t.Errorf("coordinates lost on round trip: lon=%v lat=%v", got.Longitude, got.Latitude)
	}
}

func assertRowCount(t *testing.T, pool *pgxpool.Pool, want int) {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM applications").Scan(&n); err != nil {
		t.Fatalf("count applications: %v", err)
	}
	if n != want {
		t.Fatalf("row count: got %d, want %d", n, want)
	}
}
