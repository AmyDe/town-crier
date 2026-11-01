package applications

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// PlanningApplicationID identifies a single planning application on the wire: the
// authority (the area_id rendered as a decimal string, byte-identical to the
// stored authority_code) plus the PlanIt case name. The iOS domain keys its
// PlanningApplicationId off exactly these two fields, so a single-member cluster
// can route a map-pin tap straight to the application summary sheet with no
// re-fetch.
//
// AuthoritySlug is set only by the anonymous clusters handler, whose client
// reads by slug; omitempty keeps the authed clusters response unchanged.
type PlanningApplicationID struct {
	Authority     string `json:"authority"`
	Name          string `json:"name"`
	AuthoritySlug string `json:"authoritySlug,omitempty"`
}

// Cluster is one grid-aggregated bucket of in-zone, in-viewport applications, as
// returned by FindClustersInZone. Latitude/Longitude is the centroid of the
// bucket's member points; Count is how many applications fall in the cell;
// StatusCounts is the per-app_state breakdown (it always sums to Count, with a
// NULL or unrecognised app_state folded under the "Unknown" key).
//
// Member and Members are independent:
//   - Member is set only when Count == 1, identifying the single application so the
//     client renders a real status-coloured pin and a tap opens the summary sheet.
//   - Members (JSON applicationIds) is the capped disambiguation list, populated
//     ONLY for an unsplittable multi-member cell — one whose member-point extent
//     has already collapsed below the finest (zoom-20) grid cell, so no further
//     zoom could separate the pins. For every splittable cell it is nil and the
//     wire field is omitted (omitempty), leaving today's "tap to zoom in"
//     behaviour untouched.
type Cluster struct {
	Latitude     float64                 `json:"latitude"`
	Longitude    float64                 `json:"longitude"`
	Count        int                     `json:"count"`
	StatusCounts map[string]int          `json:"statusCounts"`
	Member       *PlanningApplicationID  `json:"applicationId"`
	Members      []PlanningApplicationID `json:"applicationIds,omitempty"`
}

// ClusterQuery is the full request descriptor for FindClustersInZone: the zone
// membership circle (Latitude, Longitude, RadiusMetres — the existing ST_DWithin
// pattern), the visible map rectangle (West, South, East, North — WGS84 decimal
// degrees), the grid cell size in degrees (GridSizeDegrees — derived from the
// request's zoom by the handler, so the store stays a pure spatial primitive), an
// optional exact app_state filter (Status; "" means no status filter), and the
// coalesce threshold in degrees (CoalesceThresholdDegrees — the finest/zoom-20
// grid cell size, also supplied by the handler: a multi-member cell whose member
// points span less than this in both X and Y can never be split by zooming, so it
// carries an applicationIds member list).
type ClusterQuery struct {
	Latitude                 float64
	Longitude                float64
	RadiusMetres             float64
	West                     float64
	South                    float64
	East                     float64
	North                    float64
	GridSizeDegrees          float64
	Status                   string
	CoalesceThresholdDegrees float64
}

// clusterQueryHead and clusterQueryTail form the grid aggregation behind
// FindClustersInZone: one row per grid cell with centroid, count, per-app_state
// breakdown and, for an unsplittable multi-member cell only, the first 50 members
// by (authority, name). Output is capped at 1000 cells, densest first.
//
// $1 longitude, $2 latitude, $3 radiusMetres, $4 gridSizeDegrees, $5 west,
// $6 south, $7 east, $8 north, $9 coalesceThresholdDegrees, and (clusterQueryByStatus
// only) $10 status.
const clusterQueryHead = `WITH filtered AS (
	SELECT
		ST_SnapToGrid(location::geometry, $4) AS cell,
		COALESCE(app_state, 'Unknown') AS state,
		authority_code AS authority,
		planit_name AS name,
		location::geometry AS geom
	FROM applications
	WHERE ST_DWithin(location, ` + nearbyPoint + `, $3)
		AND location::geometry && ST_MakeEnvelope($5, $6, $7, $8, 4326)`

const clusterQueryTail = `
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
	WHERE member_count > 1 AND x_extent < $9 AND y_extent < $9
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
	clusterQueryAllStatuses = clusterQueryHead + clusterQueryTail
	clusterQueryByStatus    = clusterQueryHead + "\n\t\tAND app_state = $10" + clusterQueryTail
)

// scanClusterRow hydrates one aggregated cell. status_counts and the members list
// are read as raw JSONB and unmarshalled in Go (independent of the pgx jsonb
// codec). The single-member id is attached only for a Count == 1 cell; the member
// list is attached only when the members column is non-NULL (an unsplittable
// multi-member cell) — the two never both apply.
func scanClusterRow(row pgx.CollectableRow) (Cluster, error) {
	var (
		c          Cluster
		count      int64
		rawCounts  []byte
		authority  string
		name       string
		rawMembers []byte
	)
	if err := row.Scan(&c.Latitude, &c.Longitude, &count, &rawCounts, &authority, &name, &rawMembers); err != nil {
		return Cluster{}, err
	}
	c.Count = int(count)
	if err := json.Unmarshal(rawCounts, &c.StatusCounts); err != nil {
		return Cluster{}, fmt.Errorf("decode cluster status counts: %w", err)
	}
	if count == 1 {
		c.Member = &PlanningApplicationID{Authority: authority, Name: name}
	}
	if rawMembers != nil {
		if err := json.Unmarshal(rawMembers, &c.Members); err != nil {
			return Cluster{}, fmt.Errorf("decode cluster member list: %w", err)
		}
	}
	return c, nil
}

// FindClustersInZone returns the grid-aggregated cluster bubbles for the viewport
// q within the zone circle (q.Latitude, q.Longitude, q.RadiusMetres). Each Cluster
// is a centroid + member count + per-app_state breakdown; a single-member cell also
// carries the member's {authority, name}, and an unsplittable multi-member cell
// (member-point extent below q.CoalesceThresholdDegrees in both axes) carries the
// capped applicationIds member list. An optional q.Status restricts the whole
// aggregation — counts and member list alike — to that exact app_state. It is
// authority-agnostic and never paged: a viewport at a sane zoom yields a bounded
// number of cells (capped at 1000).
func (s *PostgresStore) FindClustersInZone(ctx context.Context, q ClusterQuery) ([]Cluster, error) {
	query := clusterQueryAllStatuses
	args := []any{q.Longitude, q.Latitude, q.RadiusMetres, q.GridSizeDegrees, q.West, q.South, q.East, q.North, q.CoalesceThresholdDegrees}
	if q.Status != "" {
		query = clusterQueryByStatus
		args = append(args, q.Status)
	}

	if s.thinMap(ctx) {
		query = thinClusterQuery(query)
	}

	var clusters []Cluster
	err := s.withNoNestLoop(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, append([]any{pgx.QueryExecModeExec}, args...)...)
		if err != nil {
			return err
		}
		clusters, err = pgx.CollectRows(rows, scanClusterRow)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("find clusters in zone near (%v, %v): %w", q.Latitude, q.Longitude, err)
	}
	return clusters, nil
}
