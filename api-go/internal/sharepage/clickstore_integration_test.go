//go:build integration

package sharepage

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func newClickStore(t *testing.T) (*PostgresClickStore, *pgxpool.Pool) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "share_page_click_totals")
	return NewPostgresClickStore(pool), pool
}

func storedClicks(t *testing.T, pool *pgxpool.Pool) []ClickTotal {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT day, authority_slug, ref, source_group, clicks
		FROM share_page_click_totals
		ORDER BY day, authority_slug, ref, source_group`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var got []ClickTotal
	for rows.Next() {
		var c ClickTotal
		var source string
		if err := rows.Scan(&c.Day, &c.AuthoritySlug, &c.Ref, &source, &c.Count); err != nil {
			t.Fatalf("scan: %v", err)
		}
		c.Source = SourceGroup(source)
		c.Day = c.Day.UTC()
		got = append(got, c)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return got
}

func TestPostgresClickStore_WriteClicks_InsertsThenIncrements(t *testing.T) {
	store, pool := newClickStore(t)
	ctx := context.Background()
	nextDay := fixedDay.AddDate(0, 0, 1)

	first := []ClickTotal{
		{Day: fixedDay, AuthoritySlug: "croydon", Ref: "23/03456/FUL", Source: SourceFacebook, Count: 2},
		{Day: fixedDay, AuthoritySlug: "croydon", Ref: "23/03456/FUL", Source: SourceNone, Count: 1},
		{Day: fixedDay, AuthoritySlug: "adur", Ref: "1", Source: SourceSEO, Count: 4},
	}
	if err := store.WriteClicks(ctx, first); err != nil {
		t.Fatalf("first write: %v", err)
	}
	second := []ClickTotal{
		{Day: fixedDay, AuthoritySlug: "croydon", Ref: "23/03456/FUL", Source: SourceFacebook, Count: 3},
		{Day: nextDay, AuthoritySlug: "croydon", Ref: "23/03456/FUL", Source: SourceFacebook, Count: 1},
		{Day: fixedDay, AuthoritySlug: "adur", Ref: "1", Source: SourceOther, Count: 1},
	}
	if err := store.WriteClicks(ctx, second); err != nil {
		t.Fatalf("second write: %v", err)
	}

	want := []ClickTotal{
		{Day: fixedDay, AuthoritySlug: "adur", Ref: "1", Source: SourceOther, Count: 1},
		{Day: fixedDay, AuthoritySlug: "adur", Ref: "1", Source: SourceSEO, Count: 4},
		{Day: fixedDay, AuthoritySlug: "croydon", Ref: "23/03456/FUL", Source: SourceFacebook, Count: 5},
		{Day: fixedDay, AuthoritySlug: "croydon", Ref: "23/03456/FUL", Source: SourceNone, Count: 1},
		{Day: nextDay, AuthoritySlug: "croydon", Ref: "23/03456/FUL", Source: SourceFacebook, Count: 1},
	}
	if got := storedClicks(t, pool); !slices.Equal(got, want) {
		t.Fatalf("stored = %+v\nwant = %+v", got, want)
	}
}

func TestPostgresClickStore_WriteClicks_EmptyIsNoOp(t *testing.T) {
	store, pool := newClickStore(t)
	if err := store.WriteClicks(context.Background(), nil); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := storedClicks(t, pool); len(got) != 0 {
		t.Fatalf("stored = %+v, want none", got)
	}
}

func TestPostgresClickStore_WriteClicks_RejectsUnknownSourceGroup(t *testing.T) {
	store, _ := newClickStore(t)
	err := store.WriteClicks(context.Background(), []ClickTotal{
		{Day: fixedDay, AuthoritySlug: "croydon", Ref: "1", Source: SourceGroup("https://example.com"), Count: 1},
	})
	if err == nil {
		t.Fatal("want the check constraint to reject a value outside the closed group")
	}
}

func TestPostgresClickStore_WriteClicks_ConcurrentReplicasAddUpWithoutDeadlock(t *testing.T) {
	store, pool := newClickStore(t)
	const keys = 50
	const replicas = 8
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errs := make(chan error, replicas)
	for replica := range replicas {
		wg.Go(func() {
			batch := make([]ClickTotal, 0, keys)
			for k := range keys {
				batch = append(batch, ClickTotal{Day: fixedDay, AuthoritySlug: "croydon", Ref: string(rune('A' + k)), Source: SourceNone, Count: 1})
			}
			if replica%2 == 1 {
				slices.Reverse(batch)
			}
			errs <- store.WriteClicks(ctx, batch)
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	got := storedClicks(t, pool)
	if len(got) != keys {
		t.Fatalf("rows = %d, want %d", len(got), keys)
	}
	for _, row := range got {
		if row.Count != replicas {
			t.Fatalf("row %+v, want count %d", row, replicas)
		}
	}
}
