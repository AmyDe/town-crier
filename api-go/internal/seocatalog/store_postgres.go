package seocatalog

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/authorities"
)

// authorityDirectory is the slice of the authority list the catalog needs.
// *authorities.Lookup satisfies it.
type authorityDirectory interface {
	All() []authorities.Authority
}

// Store maintains the SEO tables in Postgres. The gazetteer and authority
// directory are injected so tests can use a small fixture.
type Store struct {
	pool        *pgxpool.Pool
	gazetteer   Gazetteer
	authorities authorityDirectory
}

// NewStore returns a Store over the pool.
func NewStore(pool *pgxpool.Pool, gazetteer Gazetteer, dir authorityDirectory) *Store {
	return &Store{pool: pool, gazetteer: gazetteer, authorities: dir}
}

// CatalogSummary reports what a Rebuild published.
type CatalogSummary struct {
	Authorities int
	Towns       int
	Redirects   int
}

const authorityStatsQuery = `SELECT authority_code, app_state, count(*), coalesce(max(area_name), '')
FROM applications GROUP BY authority_code, app_state`

const townStatsQuery = `SELECT t.authority_code, t.slug, a.app_state, count(*)
FROM seo_towns t
JOIN applications a ON a.authority_code = t.authority_code AND ST_DWithin(a.location, t.location, $1)
GROUP BY t.authority_code, t.slug, a.app_state`

// Rebuild recomputes seo_pages and seo_redirects from the current applications
// and gazetteer. The new catalog is computed before the replacing transaction
// opens, so a computation error or a failed insert leaves the previous catalog
// untouched.
func (s *Store) Rebuild(ctx context.Context) (CatalogSummary, error) {
	if err := s.syncTowns(ctx); err != nil {
		return CatalogSummary{}, err
	}
	authorityStats, err := s.loadAuthorityStats(ctx)
	if err != nil {
		return CatalogSummary{}, err
	}
	townStats, err := s.loadTownStats(ctx)
	if err != nil {
		return CatalogSummary{}, err
	}
	catalog, err := BuildCatalog(CatalogInput{
		Authorities:    s.authorities.All(),
		Towns:          s.gazetteer.Towns,
		AuthorityStats: authorityStats,
		TownStats:      townStats,
	})
	if err != nil {
		return CatalogSummary{}, fmt.Errorf("build seo catalog: %w", err)
	}
	if err := s.replace(ctx, catalog); err != nil {
		return CatalogSummary{}, err
	}
	return summarise(catalog), nil
}

func summarise(c Catalog) CatalogSummary {
	sum := CatalogSummary{Redirects: len(c.Redirects)}
	for _, p := range c.Pages {
		switch p.Kind {
		case KindAuthority:
			sum.Authorities++
		case KindTown:
			sum.Towns++
		}
	}
	return sum
}

func (s *Store) loadAuthorityStats(ctx context.Context) (map[string]AuthorityStats, error) {
	rows, err := s.pool.Query(ctx, authorityStatsQuery)
	if err != nil {
		return nil, fmt.Errorf("query authority totals: %w", err)
	}
	defer rows.Close()
	stats := make(map[string]AuthorityStats)
	for rows.Next() {
		var (
			code, areaName string
			state          *string
			count          int
		)
		if err := rows.Scan(&code, &state, &count, &areaName); err != nil {
			return nil, fmt.Errorf("scan authority totals: %w", err)
		}
		st := stats[code]
		st.Total += count
		st.Breakdown = append(st.Breakdown, StateCount{AppState: state, Count: count})
		if areaName > st.AreaName {
			st.AreaName = areaName
		}
		stats[code] = st
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read authority totals: %w", err)
	}
	for code, st := range stats {
		sortStateCounts(st.Breakdown)
		stats[code] = st
	}
	return stats, nil
}

func (s *Store) loadTownStats(ctx context.Context) (map[TownKey]TownStats, error) {
	rows, err := s.pool.Query(ctx, townStatsQuery, float64(AssignmentRadiusMetres))
	if err != nil {
		return nil, fmt.Errorf("query town totals: %w", err)
	}
	defer rows.Close()
	stats := make(map[TownKey]TownStats)
	for rows.Next() {
		var (
			key   TownKey
			state *string
			count int
		)
		if err := rows.Scan(&key.AuthorityCode, &key.Slug, &state, &count); err != nil {
			return nil, fmt.Errorf("scan town totals: %w", err)
		}
		st := stats[key]
		st.Total += count
		st.Breakdown = append(st.Breakdown, StateCount{AppState: state, Count: count})
		stats[key] = st
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read town totals: %w", err)
	}
	for key, st := range stats {
		sortStateCounts(st.Breakdown)
		stats[key] = st
	}
	return stats, nil
}

const insertPageSQL = `INSERT INTO seo_pages
(path, kind, authority_code, town_slug, display_name, authority_name, total,
 status_breakdown, children, neighbours, lat, lng, sort_order)
VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, NULLIF($6, ''), $7, $8, $9, $10, $11, $12, $13)`

func (s *Store) replace(ctx context.Context, c Catalog) (err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin catalog replace: %w", err)
	}
	defer rollback(ctx, tx)

	if _, err = tx.Exec(ctx, "DELETE FROM seo_pages"); err != nil {
		return fmt.Errorf("clear seo_pages: %w", err)
	}
	if _, err = tx.Exec(ctx, "DELETE FROM seo_redirects"); err != nil {
		return fmt.Errorf("clear seo_redirects: %w", err)
	}

	batch := &pgx.Batch{}
	for _, p := range c.Pages {
		breakdown, merr := json.Marshal(p.StatusBreakdown)
		if merr != nil {
			return fmt.Errorf("encode breakdown for %q: %w", p.Path, merr)
		}
		children, merr := json.Marshal(p.Children)
		if merr != nil {
			return fmt.Errorf("encode children for %q: %w", p.Path, merr)
		}
		neighbours, merr := json.Marshal(p.Neighbours)
		if merr != nil {
			return fmt.Errorf("encode neighbours for %q: %w", p.Path, merr)
		}
		batch.Queue(insertPageSQL, p.Path, p.Kind, p.AuthorityCode, p.TownSlug, p.DisplayName,
			p.AuthorityName, p.Total, breakdown, children, neighbours, p.Lat, p.Lng, p.SortOrder)
	}
	for _, r := range c.Redirects {
		batch.Queue("INSERT INTO seo_redirects (path, target) VALUES ($1, $2)", r.Path, r.Target)
	}
	if err = sendBatch(ctx, tx, batch); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit catalog replace: %w", err)
	}
	return nil
}

func sendBatch(ctx context.Context, tx pgx.Tx, batch *pgx.Batch) error {
	results := tx.SendBatch(ctx, batch)
	for range batch.Len() {
		if _, err := results.Exec(); err != nil {
			results.Close() //nolint:errcheck // the Exec error is the one to report
			return fmt.Errorf("write catalog: %w", err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("write catalog: %w", err)
	}
	return nil
}

// rollback ends an uncommitted transaction. It is a no-op after Commit, and the
// rollback error is dropped because the caller already holds the error to report.
func rollback(ctx context.Context, tx pgx.Tx) {
	tx.Rollback(context.WithoutCancel(ctx)) //nolint:errcheck,contextcheck // best-effort; the transaction ends with the connection anyway
}
