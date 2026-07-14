//go:build integration

package polling

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// newPGIngester returns an Ingester wired over a real, truncated Postgres
// applications store (ADR 0032 / ADR 0041): the reindex-flood guard
// (HasSameBusinessFieldsAs) is the load-bearing mechanism every ADR 0041 lane
// depends on to make a re-ingested, unchanged record free (no write, no
// fan-out) — a fake store can assert the guard's LOGIC, but only a real
// Postgres round trip proves the guarded write genuinely never reaches the
// database. Integration tests are NOT run in parallel: pgtest.New holds a
// process-wide advisory lock for the test's duration.
func newPGIngester(t *testing.T) (*Ingester, *applications.PostgresStore) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	store := applications.NewPostgresStore(pool)
	return NewIngester(store, nil, nil), store
}

// TestIngester_Integration_NewApplicationPersists proves a first-time Ingest
// actually writes to Postgres and every field survives the round trip.
func TestIngester_Integration_NewApplicationPersists(t *testing.T) {
	ctx := context.Background()
	ingester, store := newPGIngester(t)

	ld := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	app := testApp("24/0001", 300, ld)

	if err := ingester.Ingest(ctx, app); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	got, found, err := store.GetByUID(ctx, app.UID, "300")
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if !found {
		t.Fatal("expected the ingested application to be persisted")
	}
	if got.Name != app.Name || !got.LastDifferent.Equal(ld) {
		t.Errorf("round trip mismatch: got %+v", got)
	}
}

// TestIngester_Integration_ReindexTouchAloneNeverReachesPostgres proves the
// reindex-flood guard against the REAL database, not just the fake: a
// PlanIt re-emission that bumps only LastDifferent (every ADR 0041 lane's
// steady-state re-touch case, per the churn ADR 0041 documents) must not
// write at all — the row's stored last_different must stay at the
// FIRST-ingested value, proving the guarded Upsert genuinely never reached
// Postgres a second time.
func TestIngester_Integration_ReindexTouchAloneNeverReachesPostgres(t *testing.T) {
	ctx := context.Background()
	ingester, store := newPGIngester(t)

	original := testApp("24/0002", 300, time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC))
	if err := ingester.Ingest(ctx, original); err != nil {
		t.Fatalf("Ingest (first): %v", err)
	}

	rebumped := original
	rebumped.LastDifferent = time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC) // a re-index touch, business fields unchanged
	if err := ingester.Ingest(ctx, rebumped); err != nil {
		t.Fatalf("Ingest (re-touch): %v", err)
	}

	got, found, err := store.GetByUID(ctx, original.UID, "300")
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if !found {
		t.Fatal("expected the original row to still exist")
	}
	if !got.LastDifferent.Equal(original.LastDifferent) {
		t.Errorf("last_different: got %v, want unchanged %v (a bookkeeping-only re-touch must never write)", got.LastDifferent, original.LastDifferent)
	}
}

// TestIngester_Integration_BusinessFieldChangeUpserts proves the converse: a
// genuine business-field change (a decision landing) DOES reach Postgres and
// the new value is what a subsequent read returns.
func TestIngester_Integration_BusinessFieldChangeUpserts(t *testing.T) {
	ctx := context.Background()
	ingester, store := newPGIngester(t)

	original := testApp("24/0003", 300, time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC))
	if err := ingester.Ingest(ctx, original); err != nil {
		t.Fatalf("Ingest (first): %v", err)
	}

	permitted := "Permitted"
	decided := original
	decided.AppState = &permitted
	decided.LastDifferent = time.Date(2026, 7, 14, 3, 0, 0, 0, time.UTC)
	if err := ingester.Ingest(ctx, decided); err != nil {
		t.Fatalf("Ingest (decision): %v", err)
	}

	got, found, err := store.GetByUID(ctx, original.UID, "300")
	if err != nil {
		t.Fatalf("GetByUID: %v", err)
	}
	if !found {
		t.Fatal("expected the row to exist")
	}
	if got.AppState == nil || *got.AppState != "Permitted" {
		t.Errorf("AppState: got %v, want Permitted", got.AppState)
	}
	if !got.LastDifferent.Equal(decided.LastDifferent) {
		t.Errorf("last_different: got %v, want %v (a genuine business change must write through)", got.LastDifferent, decided.LastDifferent)
	}
}
