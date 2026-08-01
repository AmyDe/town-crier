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
const findBoundaryFirstPageQuery = "SELECT " + appColumns + ", location <-> " + nearbyPoint + " AS distance " +
	"FROM applications WHERE ST_Covers(" + boundaryPolygon + ", location) " +
	"ORDER BY location <-> " + nearbyPoint + ", planit_name LIMIT $4"

// findBoundaryKeysetQuery mirrors findNearbyKeysetQuery: the identical keyset
// predicate, with the same polygon containment swap.
const findBoundaryKeysetQuery = "SELECT " + appColumns + ", location <-> " + nearbyPoint + " AS distance " +
	"FROM applications WHERE ST_Covers(" + boundaryPolygon + ", location) " +
	"AND (location <-> " + nearbyPoint + " > $5 " +
	"OR (location <-> " + nearbyPoint + " = $5 AND planit_name > $6)) " +
	"ORDER BY location <-> " + nearbyPoint + ", planit_name LIMIT $4"

// FindInBoundaryPage is FindNearbyPage's custom-shape counterpart (GH#1031,
// tc-6he3x.5): one nearest-first page of up to limit applications ST_Covers-
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

	var rows pgx.Rows
	if cursor == "" {
		rows, err = s.db.Query(ctx, findBoundaryFirstPageQuery, longitude, latitude, boundaryJSON, limit)
	} else {
		dist, name, decodeErr := decodeNearbyCursor(cursor)
		if decodeErr != nil {
			return nil, "", fmt.Errorf("decode boundary cursor: %w", decodeErr)
		}
		rows, err = s.db.Query(ctx, findBoundaryKeysetQuery, longitude, latitude, boundaryJSON, limit, dist, name)
	}
	wrap := func(err error) error {
		return fmt.Errorf("find applications in boundary near (%v, %v): %w", latitude, longitude, err)
	}
	if err != nil {
		return nil, "", wrap(err)
	}
	return collectPage(rows, scanNearbyRow, nearbyAppOf,
		func(last nearbyRow) (string, error) {
			enc, err := encodeNearbyCursor(last.dist, last.app.Name)
			if err != nil {
				return "", fmt.Errorf("encode boundary cursor: %w", err)
			}
			return enc, nil
		},
		limit, wrap)
}
