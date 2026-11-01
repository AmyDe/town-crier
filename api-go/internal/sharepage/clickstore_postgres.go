package sharepage

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

const upsertClickTotals = `
	INSERT INTO share_page_click_totals (day, authority_slug, ref, source_group, clicks)
	SELECT * FROM unnest($1::date[], $2::text[], $3::text[], $4::text[], $5::bigint[])
	ON CONFLICT (day, authority_slug, ref, source_group)
	DO UPDATE SET clicks = share_page_click_totals.clicks + EXCLUDED.clicks`

type clickExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// PostgresClickStore writes share-page click totals to Postgres.
type PostgresClickStore struct {
	db clickExecer
}

// NewPostgresClickStore returns a store backed by db.
func NewPostgresClickStore(db clickExecer) *PostgresClickStore {
	return &PostgresClickStore{db: db}
}

// WriteClicks adds each row's count to its stored total in one statement, so
// replicas can write the same keys concurrently. It reorders clicks.
func (s *PostgresClickStore) WriteClicks(ctx context.Context, clicks []ClickTotal) error {
	if len(clicks) == 0 {
		return nil
	}
	// Replicas must lock overlapping keys in one order or their upserts can deadlock.
	slices.SortFunc(clicks, func(a, b ClickTotal) int {
		return cmp.Or(
			a.Day.Compare(b.Day),
			cmp.Compare(a.AuthoritySlug, b.AuthoritySlug),
			cmp.Compare(a.Ref, b.Ref),
			cmp.Compare(a.Source, b.Source),
		)
	})

	days := make([]time.Time, len(clicks))
	slugs := make([]string, len(clicks))
	refs := make([]string, len(clicks))
	sources := make([]string, len(clicks))
	counts := make([]int64, len(clicks))
	for i, c := range clicks {
		days[i], slugs[i], refs[i], sources[i], counts[i] = c.Day, c.AuthoritySlug, c.Ref, string(c.Source), c.Count
	}
	if _, err := s.db.Exec(ctx, upsertClickTotals, days, slugs, refs, sources, counts); err != nil {
		return fmt.Errorf("upsert share click totals: %w", err)
	}
	return nil
}
