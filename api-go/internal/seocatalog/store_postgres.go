package seocatalog

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
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

// townStatsQuery reads applications once and probes the seo_towns GiST index
// per row. OFFSET 0 stops the planner flattening the LATERAL into a plain join:
// it misestimates that join and picks a per-town scan of applications, which
// on a table larger than RAM is a long run of random disk reads.
const townStatsQuery = `SELECT t.authority_code, t.slug, a.app_state, count(*), max(a.last_different)
FROM applications a
CROSS JOIN LATERAL (
  SELECT s.authority_code, s.slug FROM seo_towns s
  WHERE s.authority_code = a.authority_code AND ST_DWithin(s.location, a.location, $1)
  OFFSET 0) t
GROUP BY t.authority_code, t.slug, a.app_state`

// townListQuery returns each published town's newest TownListSize applications.
// It tests ST_Distance, not ST_DWithin: with ST_DWithin the planner fetches
// every in-radius row through the GiST index (seconds per town on a cold
// cache), whereas ST_Distance lets it walk applications_authority_real_date
// and stop at the limit.
const townListQuery = `SELECT p.path, l.rank, l.authority_code, l.planit_name
FROM unnest($1::text[], $2::text[], $3::text[]) AS p(path, authority_code, slug)
JOIN seo_towns t ON t.authority_code = p.authority_code AND t.slug = p.slug
CROSS JOIN LATERAL (
  SELECT n.authority_code, n.planit_name,
         row_number() OVER (ORDER BY ` + applications.RecentRealDateOrder + `) AS rank
  FROM (
    SELECT a.authority_code, a.planit_name, a.decided_date, a.start_date
    FROM applications a
    WHERE a.authority_code = t.authority_code AND ST_Distance(a.location, t.location) <= $4
    ORDER BY ` + applications.RecentRealDateOrder + `
    LIMIT $5) n
) l`

// Rebuild syncs seo_towns to the gazetteer, then recomputes seo_pages,
// seo_redirects and seo_town_applications from the current applications. The
// new catalog is computed before the replacing transaction opens, so a
// computation error or a failed insert leaves the previous catalog untouched.
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
	catalog.TownApplications, err = s.loadTownApplications(ctx, catalog.Pages)
	if err != nil {
		return CatalogSummary{}, err
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
	rows, err := s.pool.Query(ctx, townStatsQuery, float64(TownRadiusMetres))
	if err != nil {
		return nil, fmt.Errorf("query town totals: %w", err)
	}
	defer rows.Close()
	stats := make(map[TownKey]TownStats)
	for rows.Next() {
		var (
			key     TownKey
			state   *string
			count   int
			lastmod *time.Time
		)
		if err := rows.Scan(&key.AuthorityCode, &key.Slug, &state, &count, &lastmod); err != nil {
			return nil, fmt.Errorf("scan town totals: %w", err)
		}
		st := stats[key]
		st.Total += count
		st.Breakdown = append(st.Breakdown, StateCount{AppState: state, Count: count})
		if lastmod != nil && (st.Lastmod == nil || lastmod.After(*st.Lastmod)) {
			st.Lastmod = lastmod
		}
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

func (s *Store) loadTownApplications(ctx context.Context, pages []Page) ([]TownApplication, error) {
	var paths, codes, slugs []string
	for _, p := range pages {
		if p.Kind == KindTown {
			paths = append(paths, p.Path)
			codes = append(codes, p.AuthorityCode)
			slugs = append(slugs, p.TownSlug)
		}
	}
	if len(paths) == 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, townListQuery, paths, codes, slugs, float64(TownRadiusMetres), TownListSize)
	if err != nil {
		return nil, fmt.Errorf("query town application lists: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (TownApplication, error) {
		var ta TownApplication
		err := row.Scan(&ta.Path, &ta.Rank, &ta.AuthorityCode, &ta.PlanitName)
		return ta, err
	})
	if err != nil {
		return nil, fmt.Errorf("read town application lists: %w", err)
	}
	return out, nil
}

const insertPageSQL = `INSERT INTO seo_pages
(path, kind, authority_code, town_slug, display_name, authority_name, total,
 status_breakdown, children, neighbours, lat, lng, lastmod, sort_order)
VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, NULLIF($6, ''), $7, $8, $9, $10, $11, $12, $13, $14)`

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
	if _, err = tx.Exec(ctx, "DELETE FROM seo_town_applications"); err != nil {
		return fmt.Errorf("clear seo_town_applications: %w", err)
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
			p.AuthorityName, p.Total, breakdown, children, neighbours, p.Lat, p.Lng, p.Lastmod, p.SortOrder)
	}
	for _, r := range c.Redirects {
		batch.Queue("INSERT INTO seo_redirects (path, target) VALUES ($1, $2)", r.Path, r.Target)
	}
	for _, ta := range c.TownApplications {
		batch.Queue("INSERT INTO seo_town_applications (path, rank, authority_code, planit_name) VALUES ($1, $2, $3, $4)",
			ta.Path, ta.Rank, ta.AuthorityCode, ta.PlanitName)
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

const (
	upsertTownsSQL = `INSERT INTO seo_towns (authority_code, slug, name, population, location)
SELECT code, slug, name, population, ST_SetSRID(ST_MakePoint(lng, lat), 4326)::geography
FROM unnest($1::text[], $2::text[], $3::text[], $4::int[], $5::float8[], $6::float8[]) AS t(code, slug, name, population, lat, lng)
ON CONFLICT (authority_code, slug) DO UPDATE
SET name = EXCLUDED.name, population = EXCLUDED.population, location = EXCLUDED.location`
	deleteStaleTownsSQL = `DELETE FROM seo_towns t
WHERE NOT EXISTS (
  SELECT 1 FROM unnest($1::text[], $2::text[]) AS g(code, slug)
  WHERE g.code = t.authority_code AND g.slug = t.slug)`
)

func (s *Store) syncTowns(ctx context.Context) error {
	towns := s.gazetteer.Towns
	codes := make([]string, len(towns))
	slugs := make([]string, len(towns))
	names := make([]string, len(towns))
	populations := make([]int32, len(towns))
	lats := make([]float64, len(towns))
	lngs := make([]float64, len(towns))
	for i, t := range towns {
		codes[i] = strconv.Itoa(t.AuthorityID)
		slugs[i] = t.Slug
		names[i] = t.Name
		populations[i] = int32(t.Population)
		lats[i] = t.Lat
		lngs[i] = t.Lng
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin towns sync: %w", err)
	}
	defer rollback(ctx, tx)
	if _, err = tx.Exec(ctx, upsertTownsSQL, codes, slugs, names, populations, lats, lngs); err != nil {
		return fmt.Errorf("upsert seo_towns: %w", err)
	}
	if _, err = tx.Exec(ctx, deleteStaleTownsSQL, codes, slugs); err != nil {
		return fmt.Errorf("delete stale seo_towns: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit towns sync: %w", err)
	}
	return nil
}
