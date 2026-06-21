package applications

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// nearReadCap is the hard upper bound on documents read per town request in one
// SEO build call. The query is bounded by it (SELECT TOP @cap), so the RU cost
// and response size stay flat regardless of how busy the authority partition is.
const nearReadCap = 200

// nearDefaultRadiusMetres and nearMaxRadiusMetres bound the spatial search radius.
// radius defaults to nearDefaultRadiusMetres (a town-centroid catchment) when
// unset and is hard-clamped to nearMaxRadiusMetres, so a build request can never
// widen the single-partition scan beyond the intended town footprint.
const (
	nearDefaultRadiusMetres = 5000
	nearMaxRadiusMetres     = 10000
)

// Coordinate validity ranges for WGS84 latitude/longitude.
const (
	minLatitude  = -90
	maxLatitude  = 90
	minLongitude = -180
	maxLongitude = 180
)

// nearStore is the consumer-side store the recent-nearby SEO handler needs: a
// single bounded, single-partition spatial top-N read. The concrete *CosmosStore
// satisfies it structurally.
type nearStore interface {
	RecentNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64, cap int) ([]PlanningApplication, error)
}

// nearHandler serves the build-time town-level SEO endpoint.
type nearHandler struct {
	store  nearStore
	logger *slog.Logger
}

// NearRoutes registers the build-key-gated recent-applications-near-a-point
// endpoint that feeds town-level SEO pages. The route is anonymous to Auth0 (kept
// out of the fallback-deny set in wiring) and authenticated solely by the
// X-Build-Key gate, mirroring the sibling authority endpoint. It reads only public
// planning data from Cosmos (GH#395 Invariant 1 — never PlanIt).
func NearRoutes(mux *http.ServeMux, store nearStore, buildKey string, logger *slog.Logger) {
	h := &nearHandler{store: store, logger: logger}
	mux.HandleFunc("GET /v1/applications/near", requireBuildKey(buildKey, h.recentNearby))
}

// recentNearby returns up to limit most-recently-active applications within a
// bounded radius of (lat, lng), scoped to the required authorityId's single
// Cosmos partition, drawn from one bounded read of at most nearReadCap documents.
// authorityId scopes the partition; a missing/invalid id, a non-finite or
// out-of-range coordinate, or a non-finite/non-positive radius is a bodyless 400
// (the build key was already validated by the gate). There is no PlanIt fallback.
func (h *nearHandler) recentNearby(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// authorityId is required: it scopes the single-partition spatial query to one
	// authorityCode (the AreaID as a string), exactly as the authority endpoint does.
	id, err := strconv.Atoi(strings.TrimSpace(q.Get("authorityId")))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	lat, ok := parseCoordinate(q.Get("lat"), minLatitude, maxLatitude)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	lng, ok := parseCoordinate(q.Get("lng"), minLongitude, maxLongitude)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	radius, ok := parseRadius(q.Get("radius"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	limit := parseLimit(q.Get("limit"))
	authorityCode := strconv.Itoa(id)

	apps, err := h.store.RecentNearby(r.Context(), authorityCode, lat, lng, radius, nearReadCap)
	if err != nil {
		serverError(w, r, h.logger, "recent applications near point", err)
		return
	}

	total := len(apps)
	render := total
	if render > limit {
		render = limit
	}
	results := make([]RecentApplication, 0, render)
	for i := range render {
		results = append(results, RecentApplicationOf(apps[i]))
	}

	writeJSON(w, r, h.logger, RecentNearbyResult{
		AuthorityID:  id,
		Lat:          lat,
		Lng:          lng,
		Radius:       radius,
		Applications: results,
		Total:        total,
		TotalCapped:  total >= nearReadCap,
	})
}

// parseCoordinate parses a required WGS84 coordinate, rejecting an empty,
// non-numeric, non-finite (NaN/±Inf), or out-of-[min,max] value. The bool reports
// validity; the caller turns false into a 400.
func parseCoordinate(raw string, minVal, maxVal float64) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < minVal || v > maxVal {
		return 0, false
	}
	return v, true
}

// parseRadius parses the optional radius in metres: it defaults to
// nearDefaultRadiusMetres when unset/empty, rejects a non-numeric, non-finite, or
// non-positive value (false -> 400), and clamps anything above nearMaxRadiusMetres
// down to the hard maximum so the spatial scan stays bounded.
func parseRadius(raw string) (float64, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nearDefaultRadiusMetres, true
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0, false
	}
	if v > nearMaxRadiusMetres {
		return nearMaxRadiusMetres, true
	}
	return v, true
}
