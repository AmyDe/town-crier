package applications

import (
	"encoding/json"
	"fmt"
)

// DecodeDocument hydrates a stored Applications-container document body into a
// domain PlanningApplication, reusing the exact field and GeoJSON-point mapping
// the Cosmos read path uses (applicationDocument.toDomain). It exists so the
// Cosmos -> Postgres backfill (cmd/pgbackfill) shares one transform with the
// store rather than reinventing the mapping, which keeps the two from silently
// diverging.
//
// A document with an absent or malformed location (fewer than two coordinates)
// decodes to nil Longitude/Latitude — it is NOT rejected. That mirrors the
// store's both-or-nothing coordinate rule (newGeoPoint / coordsToLatLng) and
// lets the backfill carry coordinate-less planning records faithfully; the
// Postgres Upsert then stores a NULL location for them. An explicit [0,0] point
// is a valid coordinate and is preserved, not treated as missing.
func DecodeDocument(raw []byte) (PlanningApplication, error) {
	var doc applicationDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return PlanningApplication{}, fmt.Errorf("decode application document: %w", err)
	}
	return doc.toDomain(), nil
}
