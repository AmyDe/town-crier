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
// AuthoritySlug (GH#924 Phase 1) is populated ONLY by the anonymous clusters
// handler (anonclusters.go): the anonymous client's only point-read is the
// by-slug endpoint, and cluster members otherwise carry only the authority
// area id, not its slug. omitempty keeps the authed watch-zone clusters
// response byte-identical (that handler never sets it) — a parallel response
// type would duplicate the whole Cluster shape for one field.
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

// clusterPoint is the zone-centre query point, built from $1 (longitude) and $2
// (latitude), matching the nearbyPoint convention used elsewhere in the store.
const clusterPoint = "ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography"

// The cluster query is a single PostGIS grid aggregation, mirroring the SEO
// "GROUP BY app_state" rollup (store_postgres.go) plus ST_SnapToGrid to bucket by
// cell. It runs as a chain of CTEs within one statement:
//
//   - filtered selects the in-radius, in-viewport (and, for the by-status variant,
//     in-state) applications once, each tagged with its grid cell, app_state,
//     authority, name and point. Both downstream rollups read from it, so the
//     status filter is applied in exactly one place and the member list can never
//     name an application the filter excludes.
//   - per_cell_state pre-aggregates filtered by (grid cell, app_state): one row per
//     state present in each cell, carrying the state's count, the collected member
//     points, and the MIN (authority, name).
//   - per_cell folds those rows up per cell: the centroid is the mean of all member
//     points (ST_Centroid over the re-collected points), member_count is the SUM of
//     the per-state counts, status_counts is jsonb_object_agg over the distinct
//     per-cell states (NULL app_state folded to 'Unknown'), and the MIN
//     authority/name identify the single member when member_count = 1.
//   - cell_meta computes, per cell, the member-point extent (ST_Extent → ST_XMax -
//     ST_XMin, ST_YMax - ST_YMin) and, only for an unsplittable multi-member cell
//     (member_count > 1 AND both extent deltas below the coalesce threshold $9),
//     the capped {authority, name} member list as JSONB (jsonb_agg over a
//     deterministically ordered LIMIT-50 subquery of that cell's members). Every
//     other cell yields a NULL members column. Gating on the threshold keeps the
//     LIMIT-50 subquery off the splittable hot path, and gating on member_count > 1
//     keeps the common single-pin cell free of a redundant one-element list.
//
// Filter ($3 radius via ST_DWithin, served by the existing applications_location_gist
// GiST index) AND ($5..$8 viewport via location::geometry && ST_MakeEnvelope). Group
// by ST_SnapToGrid(location::geometry, $4) where $4 is GridSizeDegrees. A safety
// LIMIT of 1000 cells (densest first) bounds a pathological viewport.
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
	WHERE ST_DWithin(location, ` + clusterPoint + `, $3)
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
		MIN(name) AS member_name
	FROM per_cell_state
	GROUP BY cell
),
cell_meta AS (
	SELECT
		base.cell AS cell,
		CASE
			WHEN COUNT(*) > 1
				AND ST_XMax(ST_Extent(geom)) - ST_XMin(ST_Extent(geom)) < $9
				AND ST_YMax(ST_Extent(geom)) - ST_YMin(ST_Extent(geom)) < $9
			THEN (
				-- Cap the member list at 50: a single address realistically holds a
				-- handful to a couple of dozen applications, and 50 bounds the payload
				-- against a pathological centroid-geocoded mega-site. A cell exceeding
				-- 50 coincident members surfaces only the first 50 by (authority, name)
				-- — an accepted edge. jsonb_agg has no LIMIT, hence the inner subquery.
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

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find clusters in zone near (%v, %v): %w", q.Latitude, q.Longitude, err)
	}
	clusters, err := pgx.CollectRows(rows, scanClusterRow)
	if err != nil {
		return nil, fmt.Errorf("find clusters in zone near (%v, %v): %w", q.Latitude, q.Longitude, err)
	}
	return clusters, nil
}
