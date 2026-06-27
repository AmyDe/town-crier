package applications

import (
	"context"
	"errors"
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
// only when Count == 1, identifying the single application so the client renders
// a real status-coloured pin and a tap opens the summary sheet; for a multi-member
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

// FindClustersInZone is implemented in the green cycle; the stub keeps the
// package and the cmd/api wiring compiling while the handler tests drive the
// endpoint.
func (s *PostgresStore) FindClustersInZone(_ context.Context, _ ClusterQuery) ([]Cluster, error) {
	return nil, errors.New("FindClustersInZone not implemented")
}
