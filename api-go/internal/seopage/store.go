package seopage

import (
	"context"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/seocatalog"
)

// Store is the consumer-side view of the catalog and application reads the
// pages need. *PostgresStore satisfies it; cmd/api's router accepts it.
type Store interface {
	Page(ctx context.Context, path string) (seocatalog.Page, bool, error)
	RedirectTarget(ctx context.Context, path string) (string, bool, error)
	Pages(ctx context.Context, kind string) ([]seocatalog.Page, error)
	AuthorityApplications(ctx context.Context, authorityCode string, limit int) ([]applications.PlanningApplication, error)
	TownApplications(ctx context.Context, authorityCode, townSlug string, limit int) ([]applications.PlanningApplication, error)
	AuthorityLastmod(ctx context.Context, authorityCode string) (*time.Time, error)
	TownLastmod(ctx context.Context, authorityCode, townSlug string) (*time.Time, error)
	Sitemap(ctx context.Context) ([]SitemapEntry, error)
}
