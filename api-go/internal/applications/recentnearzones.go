package applications

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ZoneGeometry is a watch zone's geometry, decoupled from the watchzones
// package: applications cannot import watchzones, which already imports
// applications (see Coordinate's doc in boundarypage.go) -- so a caller
// (devseed.Seeder) translates each watchzones.WatchZone into this shape
// before calling RecentNearZones. A zero-length Boundary means a circle zone
// (Latitude, Longitude and RadiusMetres apply); a non-empty Boundary means a
// custom-shape zone (Boundary applies, the circle fields are ignored) --
// mirroring watchzones.WatchZone.IsCustomShape.
type ZoneGeometry struct {
	Latitude     float64
	Longitude    float64
	RadiusMetres float64
	Boundary     []Coordinate
}

// isCustomShape reports whether g is a custom-shape (polygon) zone rather
// than a circle, mirroring watchzones.WatchZone.IsCustomShape's zero-length
// Boundary discriminator.
func (g ZoneGeometry) isCustomShape() bool {
	return len(g.Boundary) > 0
}

// pgRecentNearZonesQuery is RecentInAuthorities' geometry-scoped replacement
// (bd tc-9nbs4.1, GH#1076 Phase 1): rather than an authority_id membership
// test, an application matches if it falls within ANY of the given zones,
// circle or polygon -- the same OR-of-circle-or-polygon shape
// watchzones.pgFindZonesContainingQuery uses for one point against every
// zone, applied the other way around: N zones against every application.
//
// The `circles` and `polygons` CTEs unnest their respective parallel-array
// parameters into per-zone rows -- the same shape pgRecentNearestTownQuery's
// `towns` CTE uses for sibling towns -- so the query text never changes shape
// however many zones of either kind are passed; a kind with zero zones
// contributes zero rows to its CTE, so its EXISTS branch is simply never
// true (never a SQL error). EXISTS, not a JOIN, is deliberate: it
// de-duplicates for free when an application falls within more than one
// zone, where a JOIN would multiply rows.
const pgRecentNearZonesQuery = `
WITH circles AS (
	SELECT ST_SetSRID(ST_MakePoint(lng, lat), 4326)::geography AS pt, radius
	FROM unnest($1::double precision[], $2::double precision[], $3::double precision[])
	     AS c(lng, lat, radius)
),
polygons AS (
	SELECT ST_GeomFromGeoJSON(g)::geography AS geom
	FROM unnest($4::text[]) AS p(g)
)
SELECT ` + appColumns + `
FROM applications a
WHERE EXISTS (SELECT 1 FROM circles c WHERE ST_DWithin(a.location, c.pt, c.radius))
   OR EXISTS (SELECT 1 FROM polygons p WHERE ST_Covers(p.geom, a.location))
ORDER BY last_different DESC
LIMIT $5`

// RecentNearZones returns up to limit most-recently-changed applications
// (last_different DESC, mirroring RecentInAuthorities' ordering) whose
// location falls within any of zones: a circle zone via ST_DWithin against
// its centre and radius, a custom-shape zone via ST_Covers against its
// polygon -- deduplicated, so an application inside more than one zone is
// returned once. It backs the dev-seed job's prod read (bd tc-9nbs4.1,
// GH#1076 Phase 1): dev's watch zones no longer need a "home" authority to
// scope the read, since notification matching is purely geographic (ADR
// 0041/0044).
//
// A nil or empty zones is a normal no-op -- mirroring RecentInAuthorities'
// nil/empty authorityIDs handling -- and returns an empty slice with no query
// executed and no error.
func (s *PostgresStore) RecentNearZones(ctx context.Context, zones []ZoneGeometry, limit int) ([]PlanningApplication, error) {
	if len(zones) == 0 {
		return []PlanningApplication{}, nil
	}

	lngs := make([]float64, 0, len(zones))
	lats := make([]float64, 0, len(zones))
	radii := make([]float64, 0, len(zones))
	polygonsGeoJSON := make([]string, 0, len(zones))
	for _, z := range zones {
		if !z.isCustomShape() {
			lngs = append(lngs, z.Longitude)
			lats = append(lats, z.Latitude)
			radii = append(radii, z.RadiusMetres)
			continue
		}
		g, err := encodeBoundaryGeoJSON(z.Boundary)
		if err != nil {
			return nil, fmt.Errorf("encode zone boundary geojson: %w", err)
		}
		polygonsGeoJSON = append(polygonsGeoJSON, g)
	}

	rows, err := s.db.Query(ctx, pgRecentNearZonesQuery, lngs, lats, radii, polygonsGeoJSON, limit)
	if err != nil {
		return nil, fmt.Errorf("recent applications near %d zones: %w", len(zones), err)
	}
	apps, err := pgx.CollectRows(rows, scanAppRow)
	if err != nil {
		return nil, fmt.Errorf("recent applications near %d zones: %w", len(zones), err)
	}
	return apps, nil
}
