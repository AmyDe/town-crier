package seopage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/seocatalog"
)

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// appReader is the applications store's recent-application reads.
type appReader interface {
	RecentByAuthority(ctx context.Context, authorityCode string, cap int) ([]applications.PlanningApplication, error)
	RecentByTown(ctx context.Context, authorityCode, townSlug string, cap int) ([]applications.PlanningApplication, error)
}

// PostgresStore reads the SEO catalog tables and delegates application lists to
// the applications store.
type PostgresStore struct {
	db   querier
	apps appReader
}

// NewPostgresStore returns a store over db (a pgx pool) and the applications
// store that serves the application lists.
func NewPostgresStore(db querier, apps appReader) *PostgresStore {
	return &PostgresStore{db: db, apps: apps}
}

const pageColumns = `path, kind, coalesce(authority_code, ''), coalesce(town_slug, ''), display_name,
	coalesce(authority_name, ''), total, status_breakdown, children, neighbours, lat, lng, sort_order`

func scanPage(row pgx.Row) (seocatalog.Page, error) {
	var p seocatalog.Page
	err := row.Scan(&p.Path, &p.Kind, &p.AuthorityCode, &p.TownSlug, &p.DisplayName, &p.AuthorityName,
		&p.Total, &p.StatusBreakdown, &p.Children, &p.Neighbours, &p.Lat, &p.Lng, &p.SortOrder)
	return p, err
}

// Page returns the catalog page at path.
func (s *PostgresStore) Page(ctx context.Context, path string) (seocatalog.Page, bool, error) {
	p, err := scanPage(s.db.QueryRow(ctx, "SELECT "+pageColumns+" FROM seo_pages WHERE path = $1", path))
	if errors.Is(err, pgx.ErrNoRows) {
		return seocatalog.Page{}, false, nil
	}
	if err != nil {
		return seocatalog.Page{}, false, fmt.Errorf("read seo page %q: %w", path, err)
	}
	return p, true, nil
}

// RedirectTarget returns the authority path a suppressed town path redirects to.
func (s *PostgresStore) RedirectTarget(ctx context.Context, path string) (string, bool, error) {
	var target string
	err := s.db.QueryRow(ctx, "SELECT target FROM seo_redirects WHERE path = $1", path).Scan(&target)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("read seo redirect %q: %w", path, err)
	}
	return target, true, nil
}

// Pages returns every catalog page of one kind in sort order.
func (s *PostgresStore) Pages(ctx context.Context, kind string) ([]seocatalog.Page, error) {
	rows, err := s.db.Query(ctx, "SELECT "+pageColumns+" FROM seo_pages WHERE kind = $1 ORDER BY sort_order", kind)
	if err != nil {
		return nil, fmt.Errorf("read seo pages of kind %q: %w", kind, err)
	}
	pages, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (seocatalog.Page, error) { return scanPage(row) })
	if err != nil {
		return nil, fmt.Errorf("read seo pages of kind %q: %w", kind, err)
	}
	return pages, nil
}

// AuthorityApplications returns the newest applications in the authority.
func (s *PostgresStore) AuthorityApplications(ctx context.Context, authorityCode string, limit int) ([]applications.PlanningApplication, error) {
	apps, err := s.apps.RecentByAuthority(ctx, authorityCode, limit)
	if err != nil {
		return nil, fmt.Errorf("authority applications: %w", err)
	}
	return apps, nil
}

// TownApplications returns the newest applications assigned to the town.
func (s *PostgresStore) TownApplications(ctx context.Context, authorityCode, townSlug string, limit int) ([]applications.PlanningApplication, error) {
	apps, err := s.apps.RecentByTown(ctx, authorityCode, townSlug, limit)
	if err != nil {
		return nil, fmt.Errorf("town applications: %w", err)
	}
	return apps, nil
}

const authorityLastmodQuery = `SELECT max(last_different) FROM applications WHERE authority_code = $1`

// AuthorityLastmod returns the newest last_different in the authority, or nil
// when it has no applications.
func (s *PostgresStore) AuthorityLastmod(ctx context.Context, authorityCode string) (*time.Time, error) {
	var t *time.Time
	if err := s.db.QueryRow(ctx, authorityLastmodQuery, authorityCode).Scan(&t); err != nil {
		return nil, fmt.Errorf("authority lastmod %q: %w", authorityCode, err)
	}
	return t, nil
}

const townLastmodQuery = `SELECT max(last_different) FROM seo_town_assignments WHERE authority_code = $1 AND town_slug = $2`

// TownLastmod returns the newest last_different among the town's assigned
// applications, or nil when none are assigned.
func (s *PostgresStore) TownLastmod(ctx context.Context, authorityCode, townSlug string) (*time.Time, error) {
	var t *time.Time
	if err := s.db.QueryRow(ctx, townLastmodQuery, authorityCode, townSlug).Scan(&t); err != nil {
		return nil, fmt.Errorf("town lastmod %q/%q: %w", authorityCode, townSlug, err)
	}
	return t, nil
}

const sitemapQuery = `SELECT p.path, p.kind, coalesce(a.last_different, t.last_different)
FROM seo_pages p
LEFT JOIN LATERAL (
	SELECT last_different FROM applications
	WHERE p.kind = 'authority' AND authority_code = p.authority_code
	ORDER BY last_different DESC LIMIT 1
) a ON true
LEFT JOIN LATERAL (
	SELECT last_different FROM seo_town_assignments
	WHERE p.kind = 'town' AND authority_code = p.authority_code AND town_slug = p.town_slug
	ORDER BY last_different DESC LIMIT 1
) t ON true
ORDER BY p.sort_order`

// Sitemap returns every catalog page in sort order with its application-set
// lastmod. Hub and towns-index rows carry no lastmod of their own.
func (s *PostgresStore) Sitemap(ctx context.Context) ([]SitemapEntry, error) {
	rows, err := s.db.Query(ctx, sitemapQuery)
	if err != nil {
		return nil, fmt.Errorf("read sitemap entries: %w", err)
	}
	entries, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (SitemapEntry, error) {
		var e SitemapEntry
		err := row.Scan(&e.Path, &e.Kind, &e.Lastmod)
		return e, err
	})
	if err != nil {
		return nil, fmt.Errorf("read sitemap entries: %w", err)
	}
	return entries, nil
}
