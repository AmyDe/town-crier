package watchzones

import (
	"context"
	"encoding/json"
	"fmt"
)

// BackfillResult reports the outcome of a one-shot location backfill: how many
// documents were scanned, how many were rewritten with a derived GeoJSON
// location, and how many already carried one (and so were left untouched).
type BackfillResult struct {
	Total      int
	Backfilled int
	AlreadyHad int
}

// backfillScanQuery selects every watch zone document across all partitions. The
// backfill is a deliberate full cross-partition scan: it is a one-shot
// maintenance task, not a hot path, and must visit every document regardless of
// its partition (/userId).
const backfillScanQuery = "SELECT * FROM c"

// BackfillLocation rewrites every WatchZone document that predates the GeoJSON
// write path (tc-x8w9) so it carries a "location" field, derived from the
// document's authoritative latitude / longitude floats. It is a guarded,
// idempotent one-shot: a document that already has a location is skipped, so a
// second run rewrites nothing. The derived point is never recomputed — Save
// re-encodes the document via newWatchZoneDocument, which builds the GeoJSON
// Point from the same persisted latitude / longitude that toDomain hydrated.
//
// This must run before FindZonesContaining switches to the index-served
// c.location query (tc-qbq4); until then the write path is additive and the
// hybrid query falls back to the inline point for un-backfilled zones, so
// nothing breaks in the meantime.
func (s *CosmosStore) BackfillLocation(ctx context.Context) (BackfillResult, error) {
	raws, err := s.items.QueryItemsCrossPartition(ctx, backfillScanQuery, nil)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("scan watch zones for backfill: %w", err)
	}

	var res BackfillResult
	for _, raw := range raws {
		res.Total++

		var doc watchZoneDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return BackfillResult{}, fmt.Errorf("decode watch zone for backfill: %w", err)
		}
		if doc.Location != nil {
			res.AlreadyHad++
			continue
		}

		zone, err := doc.toDomain()
		if err != nil {
			return BackfillResult{}, fmt.Errorf("hydrate watch zone %q for backfill: %w", doc.ID, err)
		}
		if err := s.Save(ctx, zone); err != nil {
			return BackfillResult{}, fmt.Errorf("rewrite watch zone %q with location: %w", doc.ID, err)
		}
		res.Backfilled++
	}
	return res, nil
}

// BackfillBoundingBox rewrites every WatchZone document that predates the
// bounding-box write path (tc-b179 / #637) so it carries minLat/maxLat/minLon/
// maxLon, the index-served prune the notify-path containment query runs before
// its exact ST_DISTANCE residual (replacing the dropped authority equality so
// matching is boundary-agnostic). It is a guarded, idempotent one-shot: a
// document that already has a bounding box is skipped, so a second run rewrites
// nothing. The four bbox fields are written together by newWatchZoneDocument, so
// checking minLat alone is sufficient to detect a backfilled document. The box is
// never carried over — Save re-encodes the document via newWatchZoneDocument,
// which recomputes the box from the zone's persisted centre + radius via
// WatchZone.boundingBox.
//
// It runs independently of the location backfill (BackfillLocation): a document
// may already carry a /location yet still lack a bounding box, so this can run
// after, and is safe to run after, the location backfill.
func (s *CosmosStore) BackfillBoundingBox(ctx context.Context) (BackfillResult, error) {
	raws, err := s.items.QueryItemsCrossPartition(ctx, backfillScanQuery, nil)
	if err != nil {
		return BackfillResult{}, fmt.Errorf("scan watch zones for backfill: %w", err)
	}

	var res BackfillResult
	for _, raw := range raws {
		res.Total++

		var doc watchZoneDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			return BackfillResult{}, fmt.Errorf("decode watch zone for backfill: %w", err)
		}
		if doc.MinLat != nil {
			res.AlreadyHad++
			continue
		}

		zone, err := doc.toDomain()
		if err != nil {
			return BackfillResult{}, fmt.Errorf("hydrate watch zone %q for backfill: %w", doc.ID, err)
		}
		if err := s.Save(ctx, zone); err != nil {
			return BackfillResult{}, fmt.Errorf("rewrite watch zone %q with bounding box: %w", doc.ID, err)
		}
		res.Backfilled++
	}
	return res, nil
}
