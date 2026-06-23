package admin

import (
	"context"
	"net/http"

	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// watchZoneBackfiller is the consumer-side view of the watch-zone store the
// backfill endpoint needs: a single one-shot rewrite of every document missing a
// GeoJSON location. watchzones.CosmosStore satisfies it.
type watchZoneBackfiller interface {
	BackfillLocation(ctx context.Context) (watchzones.BackfillResult, error)
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
