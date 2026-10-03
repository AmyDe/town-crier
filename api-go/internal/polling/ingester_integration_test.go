//go:build integration

package polling

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func newPGEventIngester(t *testing.T) (*Ingester, *pgxpool.Pool, *applications.PostgresStore) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "application_event")
	return NewPostgresIngester(pool), pool, applications.NewPostgresStore(pool)
}

func eventKinds(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT kind FROM application_event ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		out = append(out, k)
	}
	return out
}

func TestEventIngester_Integration_Cases(t *testing.T) {
	ctx := context.Background()
	ing, pool, _ := newPGEventIngester(t)
	step := func(name string, app applications.PlanningApplication, want []string) {
		t.Helper()
		if err := ing.Ingest(ctx, app); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := eventKinds(t, pool); !equalStrings(got, want) {
			t.Fatalf("%s: events = %v, want %v", name, got, want)
		}
	}

	app := evApp("Undecided", day(1), nil)
	step("new", app, []string{"new_application"})
	step("unchanged", app, []string{"new_application"})

	ref := "r2"
	silent := app
	silent.Reference = &ref
	step("silent only", silent, []string{"new_application"})

	desc := silent
	desc.Description = "changed"
	step("non-decision field change", desc, []string{"new_application"})

	permitted := evApp("Permitted", day(1), day(5))
	permitted.Reference = &ref
	permitted.Description = "changed"
	step("transition", permitted, []string{"new_application", "decision"})

	rejected := permitted
	rej := "Rejected"
	rejected.AppState = &rej
	step("decision to decision", rejected, []string{"new_application", "decision"})

	var d string
	if err := pool.QueryRow(ctx, "SELECT event_date::text FROM application_event WHERE kind='decision'").Scan(&d); err != nil {
		t.Fatal(err)
	}
	if d != "2026-06-05" {
		t.Errorf("decision event_date = %s", d)
	}
	var s string
	if err := pool.QueryRow(ctx, "SELECT event_date::text FROM application_event WHERE kind='new_application'").Scan(&s); err != nil {
		t.Fatal(err)
	}
	if s != "2026-06-01" {
		t.Errorf("new_application event_date = %s", s)
	}
}

func TestEventIngester_Integration_NewDecidedEmitsBoth(t *testing.T) {
	ing, pool, _ := newPGEventIngester(t)
	if err := ing.Ingest(context.Background(), evApp("Conditions", day(1), day(3))); err != nil {
		t.Fatal(err)
	}
	if got := eventKinds(t, pool); !equalStrings(got, []string{"new_application", "decision"}) {
		t.Fatalf("events = %v", got)
	}
}

type failingEventUnit struct{ ingestUnit }

func (failingEventUnit) InsertEvent(context.Context, ApplicationEvent) error {
	return errors.New("boom")
}

type failingOpener struct{ inner ingestUnitOpener }

func (o failingOpener) Begin(ctx context.Context) (ingestUnit, error) {
	u, err := o.inner.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return failingEventUnit{u}, nil
}

func TestEventIngester_Integration_RollbackLeavesNeitherRowNorEvent(t *testing.T) {
	ctx := context.Background()
	_, pool, store := newPGEventIngester(t)
	ing := NewIngester(failingOpener{pgIngestOpener{pool: pool}})
	app := evApp("Undecided", day(1), nil)
	if err := ing.Ingest(ctx, app); err == nil {
		t.Fatal("expected error")
	}
	if _, found, err := store.GetByUID(ctx, app.UID, "300"); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if got := eventKinds(t, pool); len(got) != 0 {
		t.Fatalf("events = %v", got)
	}
}
