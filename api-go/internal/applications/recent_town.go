package applications

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const pgRecentByTownQuery = "SELECT " + appColumns +
	" FROM applications WHERE authority_code = $1 AND EXISTS (" +
	"SELECT 1 FROM seo_town_assignments s WHERE s.authority_code = applications.authority_code " +
	"AND s.planit_name = applications.planit_name AND s.town_slug = $2) " +
	"ORDER BY " + recentRealDateOrder + " LIMIT $3"

// RecentByTown returns up to cap most-recently-active applications assigned to
// the town (seo_town_assignments), ordered by recentRealDateOrder. It backs the
// live SEO town page.
func (s *PostgresStore) RecentByTown(ctx context.Context, authorityCode, townSlug string, cap int) ([]PlanningApplication, error) {
	rows, err := s.db.Query(ctx, pgRecentByTownQuery, authorityCode, townSlug, cap)
	if err != nil {
		return nil, fmt.Errorf("recent applications for town %q/%q: %w", authorityCode, townSlug, err)
	}
	apps, err := pgx.CollectRows(rows, scanAppRow)
	if err != nil {
		return nil, fmt.Errorf("recent applications for town %q/%q: %w", authorityCode, townSlug, err)
	}
	return apps, nil
}
