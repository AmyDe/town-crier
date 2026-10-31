package applications

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// BoundaryClusterQuery is FindClustersInBoundary's request descriptor: the custom-shape zone's polygon (Boundary) replaces
// ClusterQuery's membership circle (Latitude, Longitude, RadiusMetres); the
// viewport, grid size, status filter and coalesce threshold carry the same
// meaning as ClusterQuery's.
type BoundaryClusterQuery struct {
	Boundary                 []Coordinate
	West                     float64
	South                    float64
	East                     float64
	North                    float64
	GridSizeDegrees          float64
	Status                   string
	CoalesceThresholdDegrees float64
}

// boundaryClusterQueryHead mirrors clusterQueryHead: identical grid-cell
// tagging and viewport predicate, with ST_Covers against the polygon replacing
// ST_DWithin against the circle. $2 is the grid size, $3..$6 the viewport.
const boundaryClusterQueryHead = `WITH filtered AS (
	SELECT
		ST_SnapToGrid(location::geometry, $2) AS cell,
		COALESCE(app_state, 'Unknown') AS state,
		authority_code AS authority,
		planit_name AS name,
		location::geometry AS geom
	FROM applications
	WHERE ST_Covers(` + boundaryZonePolygonNoPoint + `, location)
		AND location::geometry && ST_MakeEnvelope($3, $4, $5, $6, 4326)`

// boundaryClusterQueryTail is clusterQueryTail renumbered for this query's
// parameters: $7 is the coalesce threshold and, for boundaryClusterQueryByStatus
// only, $8 the status filter.
const boundaryClusterQueryTail = `
),
per_cell_state AS (
	SELECT
		cell,
		state,
		COUNT(*) AS n,
		ST_Collect(geom) AS geoms,
		MIN(authority) AS authority,
		MIN(name) AS name
	FROM filtered
	GROUP BY cell, state
),
per_cell AS (
	SELECT
		cell,
		ST_Y(ST_Centroid(ST_Collect(geoms))) AS centroid_lat,
		ST_X(ST_Centroid(ST_Collect(geoms))) AS centroid_lon,
		SUM(n)::bigint AS member_count,
		jsonb_object_agg(state, n) AS status_counts,
		MIN(authority) AS member_authority,
		MIN(name) AS member_name,
		ST_XMax(ST_Extent(geoms)) - ST_XMin(ST_Extent(geoms)) AS x_extent,
		ST_YMax(ST_Extent(geoms)) - ST_YMin(ST_Extent(geoms)) AS y_extent
	FROM per_cell_state
	GROUP BY cell
),
unsplittable AS (
	SELECT cell FROM per_cell
	WHERE member_count > 1 AND x_extent < $7 AND y_extent < $7
),
ranked AS (
	SELECT f.cell, f.authority, f.name,
		row_number() OVER (PARTITION BY f.cell ORDER BY f.authority, f.name) AS rn
	FROM filtered f
	JOIN unsplittable u ON u.cell = f.cell
),
members AS (
	SELECT cell,
		jsonb_agg(jsonb_build_object('authority', authority, 'name', name) ORDER BY rn) AS members
	FROM ranked
	WHERE rn <= 50
	GROUP BY cell
)
SELECT
	per_cell.centroid_lat,
	per_cell.centroid_lon,
	per_cell.member_count,
	per_cell.status_counts,
	per_cell.member_authority,
	per_cell.member_name,
	members.members
FROM per_cell
LEFT JOIN members ON members.cell = per_cell.cell
ORDER BY per_cell.member_count DESC
LIMIT 1000`

const (
	boundaryClusterQueryAllStatuses = boundaryClusterQueryHead + boundaryClusterQueryTail
	boundaryClusterQueryByStatus    = boundaryClusterQueryHead + "\n\t\tAND app_state = $8" + boundaryClusterQueryTail
)

// FindClustersInBoundary is FindClustersInZone's custom-shape counterpart:
// the same Cluster rows, restricted to applications ST_Covers-contained in
// q.Boundary rather than within a circle.
func (s *PostgresStore) FindClustersInBoundary(ctx context.Context, q BoundaryClusterQuery) ([]Cluster, error) {
	boundaryJSON, err := encodeBoundaryGeoJSON(q.Boundary)
	if err != nil {
		return nil, fmt.Errorf("encode boundary geojson: %w", err)
	}

	query := boundaryClusterQueryAllStatuses
	args := []any{boundaryJSON, q.GridSizeDegrees, q.West, q.South, q.East, q.North, q.CoalesceThresholdDegrees}
	if q.Status != "" {
		query = boundaryClusterQueryByStatus
		args = append(args, q.Status)
	}

	var clusters []Cluster
	err = s.withNoNestLoop(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, append([]any{pgx.QueryExecModeExec}, args...)...)
		if err != nil {
			return err
		}
		clusters, err = pgx.CollectRows(rows, scanClusterRow)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("find clusters in boundary: %w", err)
	}
	return clusters, nil
}
