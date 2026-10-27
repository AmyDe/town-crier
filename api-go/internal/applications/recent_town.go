package applications

import (
	"context"
	"fmt"
)

const pgCatalogTownQuery = "SELECT " + appColumns +
	" FROM seo_town_applications s JOIN applications a USING (authority_code, planit_name)" +
	" WHERE s.path = $1 ORDER BY s.rank"

// CatalogTownApplications returns the applications the seo-catalog job
// precomputed for the town page at path ("<authority>/<town>"), in rank order,
// with their current state. It backs the live SEO town page.
func (s *PostgresStore) CatalogTownApplications(ctx context.Context, path string) ([]PlanningApplication, error) {
	apps, err := s.queryApps(ctx, pgCatalogTownQuery, path)
	if err != nil {
		return nil, fmt.Errorf("catalog applications for town %q: %w", path, err)
	}
	return apps, nil
}
