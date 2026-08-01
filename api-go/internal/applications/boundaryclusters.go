package applications

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// BoundaryClusterQuery is FindClustersInBoundary's request descriptor
// (GH#1031, tc-6he3x.5): the custom-shape zone's polygon (Boundary) replaces
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

// boundaryClusterPolygon is the custom-shape zone's polygon, built from $1 --
// the GeoJSON-encoded ring. It is the boundary-scoped counterpart to
// clusterPoint's circle, at a different parameter position (this query has no
// centre/radius, so the polygon takes the leading placeholder).
const boundaryClusterPolygon = "ST_GeomFromGeoJSON($1)::geography"

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
	WHERE ST_Covers(` + boundaryClusterPolygon + `, location)
		AND location::geometry && ST_MakeEnvelope($3, $4, $5, $6, 4326)`

// boundaryClusterQueryTail mirrors clusterQueryTail exactly, renumbered for
// this query's parameter positions: $7 is the coalesce threshold (clusterTail's
// $9) and, for boundaryClusterQueryByStatus only, $8 is the status filter
// (clusterTail's $10). See clusterQueryTail for the full per-cell aggregation
// rationale, which applies unchanged here.
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
		MIN(name) AS member_name
	FROM per_cell_state
	GROUP BY cell
),
cell_meta AS (
	SELECT
		base.cell AS cell,
		CASE
			WHEN COUNT(*) > 1
				AND ST_XMax(ST_Extent(geom)) - ST_XMin(ST_Extent(geom)) < $7
				AND ST_YMax(ST_Extent(geom)) - ST_YMin(ST_Extent(geom)) < $7
			THEN (
				SELECT jsonb_agg(jsonb_build_object('authority', m.authority, 'name', m.name))
				FROM (
					SELECT authority, name
					FROM filtered f
					WHERE f.cell = base.cell
					ORDER BY authority, name
					LIMIT 50
				) m
			)
			ELSE NULL
		END AS members
	FROM filtered base
	GROUP BY base.cell
)
SELECT
	per_cell.centroid_lat,
	per_cell.centroid_lon,
	per_cell.member_count,
	per_cell.status_counts,
	per_cell.member_authority,
	per_cell.member_name,
	cell_meta.members
FROM per_cell
JOIN cell_meta ON cell_meta.cell = per_cell.cell
ORDER BY per_cell.member_count DESC
LIMIT 1000`

const (
	boundaryClusterQueryAllStatuses = boundaryClusterQueryHead + boundaryClusterQueryTail
	boundaryClusterQueryByStatus    = boundaryClusterQueryHead + "\n\t\tAND app_state = $8" + boundaryClusterQueryTail
)

// FindClustersInBoundary is FindClustersInZone's custom-shape counterpart
// (GH#1031, tc-6he3x.5): grid-aggregated cluster bubbles for the viewport q,
// restricted to applications ST_Covers-contained in q.Boundary rather than
// within a circle. The cell/centroid/status-breakdown/member-list shape is
// identical to FindClustersInZone's Cluster rows; see FindClustersInZone and
// clusterQueryHead/clusterQueryTail for the full aggregation rationale, which
// applies unchanged here -- only the containment predicate differs.
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

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find clusters in boundary: %w", err)
	}
	clusters, err := pgx.CollectRows(rows, scanClusterRow)
	if err != nil {
		return nil, fmt.Errorf("find clusters in boundary: %w", err)
	}
	return clusters, nil
}
