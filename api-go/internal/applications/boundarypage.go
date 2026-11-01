package applications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Coordinate is a single lon/lat vertex of a custom-shape watch-zone boundary
// polygon, GeoJSON-ordered (longitude first) to match watchzones.Boundary's
// wire/store encoding (watchzones/store_postgres.go's encodeBoundaryGeoJSON)
// and what PostGIS's ST_GeomFromGeoJSON expects. It is this package's own
// type -- applications cannot import watchzones, which already imports
// applications -- so a caller (watchzones/nearby.go) converts its own
// Boundary/Coordinate ring into a []Coordinate of this shape before calling
// FindInBoundaryPage or FindClustersInBoundary.
type Coordinate struct {
	Longitude float64
	Latitude  float64
}

// boundaryGeoJSON is the GeoJSON Polygon shape ST_GeomFromGeoJSON(...) expects:
// a single exterior ring, [longitude, latitude] coordinate order -- byte-for-
// byte the same shape watchzones/store_postgres.go's pgBoundaryGeoJSON
// produces, so a zone's persisted boundary and the ad hoc boundary query
// parameter here encode identically.
type boundaryGeoJSON struct {
	Type        string         `json:"type"`
	Coordinates [][][2]float64 `json:"coordinates"`
}

// errEmptyBoundary signals a caller bug: FindInBoundaryPage and
// FindClustersInBoundary are reached only for a custom-shape zone, whose
// Boundary is guaranteed non-empty by watchzones.NewBoundary.
var errEmptyBoundary = errors.New("boundary must not be empty")

// encodeBoundaryGeoJSON renders ring as GeoJSON Polygon text for
// ST_GeomFromGeoJSON($n)::geography.
func encodeBoundaryGeoJSON(ring []Coordinate) (string, error) {
	if len(ring) == 0 {
		return "", errEmptyBoundary
	}
	coords := make([][2]float64, len(ring))
	for i, v := range ring {
		coords[i] = [2]float64{v.Longitude, v.Latitude}
	}
	raw, err := json.Marshal(boundaryGeoJSON{Type: "Polygon", Coordinates: [][][2]float64{coords}})
	if err != nil {
		return "", fmt.Errorf("encode boundary geojson: %w", err)
	}
	return string(raw), nil
}

// boundaryPolygon is the custom-shape zone's polygon, built from $3 -- the
// GeoJSON-encoded ring ST_GeomFromGeoJSON parses back into a geography. It
// pairs with nearbyPoint ($1 longitude, $2 latitude -- the zone's derived
// centroid, supplied by the caller so distance-ordering and its keyset cursor
// stay byte-identical in shape to FindNearbyPage's) for the KNN ORDER BY.
const boundaryPolygon = "ST_GeomFromGeoJSON($3)::geography"

// findBoundaryFirstPageQuery mirrors findNearbyFirstPageQuery: the same
// nearest-centroid-first KNN ordering and (distance, planit_name) keyset shape,
// with ST_Covers against the polygon replacing ST_DWithin against the circle.
var findBoundaryFirstPageQuery = knnThenExact("ST_Covers("+boundaryPolygon+", location)", "$4")

// findBoundaryKeysetQuery mirrors findNearbyKeysetQuery: the identical keyset
// predicate, with the same polygon containment swap.
var findBoundaryKeysetQuery = knnThenExact("ST_Covers("+boundaryPolygon+", location) "+
	"AND (location <-> "+nearbyPoint+" > $5 "+
	"OR (location <-> "+nearbyPoint+" = $5 AND planit_name > $6))", "$4")

// FindInBoundaryPage is FindNearbyPage's custom-shape counterpart: one nearest-first page of up to limit applications ST_Covers-
// contained in boundary (a closed polygon ring, [longitude, latitude]-ordered),
// plus an opaque continuation cursor byte-identical in shape to
// FindNearbyPage's. latitude and longitude are the zone's derived centroid
// (see watchzones.WatchZone.WithBoundary) and are used ONLY to order the page
// nearest-centroid-first -- containment is decided entirely by ST_Covers
// against the polygon, never by the centroid or any implied radius.
func (s *PostgresStore) FindInBoundaryPage(ctx context.Context, latitude, longitude float64, boundary []Coordinate, limit int, cursor string) ([]PlanningApplication, string, error) {
	boundaryJSON, err := encodeBoundaryGeoJSON(boundary)
	if err != nil {
		return nil, "", fmt.Errorf("encode boundary geojson: %w", err)
	}

	query, args := findBoundaryFirstPageQuery, []any{longitude, latitude, boundaryJSON, limit}
	if cursor != "" {
		dist, name, decodeErr := decodeNearbyCursor(cursor)
		if decodeErr != nil {
			return nil, "", fmt.Errorf("decode boundary cursor: %w", decodeErr)
		}
		query, args = findBoundaryKeysetQuery, append(args, dist, name)
	}
	wrap := func(err error) error {
		return fmt.Errorf("find applications in boundary near (%v, %v): %w", latitude, longitude, err)
	}
	if s.thinList(ctx) {
		thinQuery := thinBoundaryFirstQuery
		if cursor != "" {
			thinQuery = thinBoundaryKeysetQuery
		}
		return s.pageThin(ctx, s.withKNNWalk, thinQuery, args, SortDistance, limit, func(r thinRow) (string, error) {
			enc, err := encodeNearbyCursor(r.dist, r.key.name)
			if err != nil {
				return "", fmt.Errorf("encode boundary cursor: %w", err)
			}
			return enc, nil
		}, wrap)
	}
	var (
		apps []PlanningApplication
		next string
	)
	err = s.withKNNWalk(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, query, append([]any{pgx.QueryExecModeExec}, args...)...)
		if err != nil {
			return wrap(err)
		}
		apps, next, err = collectPage(rows, scanNearbyRow, nearbyAppOf,
			func(last nearbyRow) (string, error) {
				enc, err := encodeNearbyCursor(last.dist, last.app.Name)
				if err != nil {
					return "", fmt.Errorf("encode boundary cursor: %w", err)
				}
				return enc, nil
			},
			limit, wrap)
		return err
	})
	if err != nil {
		return nil, "", err
	}
	return apps, next, nil
}
