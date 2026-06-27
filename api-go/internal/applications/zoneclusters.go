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
type PlanningApplicationID struct {
	Authority string `json:"authority"`
	Name      string `json:"name"`
}

// Cluster is one grid-aggregated bucket of in-zone, in-viewport applications, as
// returned by FindClustersInZone. Latitude/Longitude is the centroid of the
// bucket's member points; Count is how many applications fall in the cell;
// StatusCounts is the per-app_state breakdown (it always sums to Count, with a
// NULL or unrecognised app_state folded under the "Unknown" key). Member is set
// only when Count == 1, identifying the single application so the client renders a
// real status-coloured pin and a tap opens the summary sheet; for a multi-member
// cell it is nil and the client renders a count bubble.
type Cluster struct {
	Latitude     float64                `json:"latitude"`
	Longitude    float64                `json:"longitude"`
	Count        int                    `json:"count"`
	StatusCounts map[string]int         `json:"statusCounts"`
	Member       *PlanningApplicationID `json:"applicationId"`
}

// ClusterQuery is the full request descriptor for FindClustersInZone: the zone
// membership circle (Latitude, Longitude, RadiusMetres — the existing ST_DWithin
// pattern), the visible map rectangle (West, South, East, North — WGS84 decimal
// degrees), the grid cell size in degrees (GridSizeDegrees — derived from the
// request's zoom by the handler, so the store stays a pure spatial primitive),
// and an optional exact app_state filter (Status; "" means no status filter).
type ClusterQuery struct {
	Latitude        float64
	Longitude       float64
	RadiusMetres    float64
	West            float64
	South           float64
	East            float64
	North           float64
	GridSizeDegrees float64
	Status          string
}

// clusterPoint is the zone-centre query point, built from $1 (longitude) and $2
// (latitude), matching the nearbyPoint convention used elsewhere in the store.
const clusterPoint = "ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography"

// The cluster query is a single PostGIS grid aggregation, mirroring the SEO
// "GROUP BY app_state" rollup (store_postgres.go) plus ST_SnapToGrid to bucket by
// cell. It runs in two stages within one statement:
//
//   - per_cell_state pre-aggregates the in-radius, in-viewport applications by
//     (grid cell, app_state): one row per state present in each cell, carrying the
//     state's count, the collected member points, and the MIN (authority_code,
//     planit_name) — which, for a single-member cell, is exactly that member.
//   - the outer SELECT folds those rows up per cell: the centroid is the mean of
//     all member points (ST_Centroid over the re-collected points), member_count
//     is the SUM of the per-state counts, status_counts is jsonb_object_agg over
//     the distinct per-cell states (NULL app_state folded to 'Unknown', so the
//     object never has a NULL key and always sums to member_count), and the MIN
//     authority/name identify the single member when member_count = 1.
//
// Filter ($3 radius via ST_DWithin, served by the existing applications_location_gist
// GiST index) AND ($5..$8 viewport via location::geometry && ST_MakeEnvelope).
// Group by ST_SnapToGrid(location::geometry, $4) where $4 is GridSizeDegrees. A
// safety LIMIT of 1000 cells (densest first) bounds a pathological viewport.
//
// $1 longitude, $2 latitude, $3 radiusMetres, $4 gridSizeDegrees,
// $5 west, $6 south, $7 east, $8 north, and (clusterQueryByStatus only) $9 status.
const clusterQueryHead = `WITH per_cell_state AS (
	SELECT
		ST_SnapToGrid(location::geometry, $4) AS cell,
		COALESCE(app_state, 'Unknown') AS state,
		COUNT(*) AS n,
		ST_Collect(location::geometry) AS geoms,
		MIN(authority_code) AS authority,
		MIN(planit_name) AS name
	FROM applications
	WHERE ST_DWithin(location, ` + clusterPoint + `, $3)
		AND location::geometry && ST_MakeEnvelope($5, $6, $7, $8, 4326)`

const clusterQueryTail = `
	GROUP BY cell, COALESCE(app_state, 'Unknown')
)
SELECT
	ST_Y(ST_Centroid(ST_Collect(geoms))) AS centroid_lat,
	ST_X(ST_Centroid(ST_Collect(geoms))) AS centroid_lon,
	SUM(n)::bigint AS member_count,
	jsonb_object_agg(state, n) AS status_counts,
	MIN(authority) AS member_authority,
	MIN(name) AS member_name
FROM per_cell_state
GROUP BY cell
ORDER BY member_count DESC
LIMIT 1000`

const (
	clusterQueryAllStatuses = clusterQueryHead + clusterQueryTail
	clusterQueryByStatus    = clusterQueryHead + "\n\t\tAND app_state = $9" + clusterQueryTail
)

// scanClusterRow hydrates one aggregated cell. status_counts is read as raw JSONB
// and unmarshalled in Go (independent of the pgx jsonb codec). The member id is
// attached only for a single-member cell.
func scanClusterRow(row pgx.CollectableRow) (Cluster, error) {
	var (
		c         Cluster
		count     int64
		rawCounts []byte
		authority string
		name      string
	)
	if err := row.Scan(&c.Latitude, &c.Longitude, &count, &rawCounts, &authority, &name); err != nil {
		return Cluster{}, err
	}
	c.Count = int(count)
	if err := json.Unmarshal(rawCounts, &c.StatusCounts); err != nil {
		return Cluster{}, fmt.Errorf("decode cluster status counts: %w", err)
	}
	if count == 1 {
		c.Member = &PlanningApplicationID{Authority: authority, Name: name}
	}
	return c, nil
}

// FindClustersInZone returns the grid-aggregated cluster bubbles for the viewport
// q within the zone circle (q.Latitude, q.Longitude, q.RadiusMetres). Each Cluster
// is a centroid + member count + per-app_state breakdown; a single-member cell also
// carries the member's {authority, name}. An optional q.Status restricts the
// aggregation to that exact app_state. It is authority-agnostic and never paged:
// a viewport at a sane zoom yields a bounded number of cells (capped at 1000).
func (s *PostgresStore) FindClustersInZone(ctx context.Context, q ClusterQuery) ([]Cluster, error) {
	query := clusterQueryAllStatuses
	args := []any{q.Longitude, q.Latitude, q.RadiusMetres, q.GridSizeDegrees, q.West, q.South, q.East, q.North}
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
