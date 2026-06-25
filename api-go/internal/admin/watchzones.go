package admin

import (
	"context"
	"net/http"

	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// watchZoneBackfiller is the consumer-side view of the watch-zone store the
// backfill endpoints need: one-shot rewrites of every document missing a derived
// field — a GeoJSON location, or a bounding box. watchzones.CosmosStore satisfies
// it.
type watchZoneBackfiller interface {
	BackfillLocation(ctx context.Context) (watchzones.BackfillResult, error)
	BackfillBoundingBox(ctx context.Context) (watchzones.BackfillResult, error)
}

// backfillResponse is the response shape for the backfill endpoint:
// { total, backfilled, alreadyHad }, all camelCase.
type backfillResponse struct {
	Total      int `json:"total"`
	Backfilled int `json:"backfilled"`
	AlreadyHad int `json:"alreadyHad"`
}

// backfillWatchZoneLocation implements POST /v1/admin/watchzones/backfill-location:
// a guarded, idempotent one-shot that rewrites every WatchZone document predating
// the GeoJSON write path so it carries a /location field (tc-xj48). The request
// body is ignored; the reconciled counts are returned as JSON. This must run
// before FindZonesContaining switches to the index-served c.location query.
func (h *handler) backfillWatchZoneLocation(w http.ResponseWriter, r *http.Request) {
	res, err := h.watchZones.BackfillLocation(r.Context())
	if err != nil {
		h.serverError(w, r, "backfill watch zone location", err)
		return
	}
	h.writeJSON(r, w, backfillResponse{Total: res.Total, Backfilled: res.Backfilled, AlreadyHad: res.AlreadyHad})
}

// backfillWatchZoneBoundingBox implements POST /v1/admin/watchzones/backfill-bbox:
// a guarded, idempotent one-shot that rewrites every WatchZone document predating
// the bounding-box write path so it carries minLat/maxLat/minLon/maxLon (tc-b179 /
// #637), recomputed from the zone's centre + radius. The request body is ignored;
// the reconciled counts are returned as JSON. It runs independently of (and may
// run after) the location backfill.
func (h *handler) backfillWatchZoneBoundingBox(w http.ResponseWriter, r *http.Request) {
	res, err := h.watchZones.BackfillBoundingBox(r.Context())
	if err != nil {
		h.serverError(w, r, "backfill watch zone bounding box", err)
		return
	}
	h.writeJSON(r, w, backfillResponse{Total: res.Total, Backfilled: res.Backfilled, AlreadyHad: res.AlreadyHad})
}
