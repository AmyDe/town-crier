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

// nearStore is the consumer-side store the recent-nearby SEO handler needs: two
// bounded, single-partition spatial top-N reads (recency-ordered and
// distance-ordered) plus a whole-in-radius per-appState breakdown (whose buckets
// sum to the exact in-radius total). The concrete *CosmosStore satisfies it
// structurally.
type nearStore interface {
	RecentNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64, cap int) ([]PlanningApplication, error)
	NearestNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64, cap int) ([]PlanningApplication, error)
	BreakdownNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64) ([]StateCount, error)
}

// near ordering modes for the optional order query param. recency (the default,
// also selected by an absent param) orders by lastDifferent DESC via
// RecentNearby; distance orders by ST_DISTANCE ASC (nearest first) via
// NearestNearby. Any other value is a 400.
const (
	orderRecency  = "recency"
	orderDistance = "distance"
)

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

// recentNearby returns up to limit applications within a bounded radius of (lat,
// lng), scoped to the required authorityId's single Cosmos partition, drawn from
// one bounded read of at most nearReadCap documents. The optional order param
// selects the ordering: recency (default, also when absent) is most-recently-
// active first; distance is nearest-first (for overlap reduction between adjacent
// town pages). authorityId scopes the partition; a missing/invalid id, a
// non-finite or out-of-range coordinate, a non-finite/non-positive radius, or an
// unknown order value is a bodyless 400 (the build key was already validated by
// the gate). There is no PlanIt fallback.
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

	// nearby selects the ordering: distance -> NearestNearby (nearest first),
	// recency or absent -> RecentNearby (lastDifferent DESC, unchanged default).
	nearby, ok := selectNearbyRead(h.store, q.Get("order"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	limit := parseLimit(q.Get("limit"))
	authorityCode := strconv.Itoa(id)

	apps, err := nearby(r.Context(), authorityCode, lat, lng, radius, nearReadCap)
	if err != nil {
		serverError(w, r, h.logger, "recent applications near point", err)
		return
	}

	// breakdown is the per-appState distribution over the WHOLE in-radius set (an
	// index-served spatial GROUP BY), independent of the bounded read which
	// saturates at nearReadCap. The render slice must clamp against the bounded
	// read length, NOT the breakdown total — the total can dwarf len(apps).
	breakdown, err := h.store.BreakdownNearby(r.Context(), authorityCode, lat, lng, radius)
	if err != nil {
		serverError(w, r, h.logger, "status breakdown near point", err)
		return
	}

	// StatusBreakdown is always a non-null array on the wire; nothing in radius
	// yields an empty (not nil) slice so it marshals to [] rather than null.
	if breakdown == nil {
		breakdown = []StateCount{}
	}

	// total is the EXACT whole-in-radius total: the sum of the breakdown buckets,
	// so the rendered "tracking N applications" lead line equals the breakdown.
	total := 0
	for _, sc := range breakdown {
		total += sc.Count
	}

	render := len(apps)
	if render > limit {
		render = limit
	}
	results := make([]RecentApplication, 0, render)
	for i := range render {
		results = append(results, RecentApplicationOf(apps[i]))
	}

	writeJSON(w, r, h.logger, RecentNearbyResult{
		AuthorityID:     id,
		Lat:             lat,
		Lng:             lng,
		Radius:          radius,
		Applications:    results,
		Total:           total,
		StatusBreakdown: breakdown,
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

// selectNearbyRead maps the optional order query param to the bounded
// single-partition read it selects: an empty or "recency" value picks the
// recency-ordered RecentNearby (the unchanged default), "distance" picks the
// distance-ordered NearestNearby (nearest first). The bool reports validity; the
// caller turns false into a 400. Both reads share the same signature, so the
// handler calls the result without branching further.
func selectNearbyRead(store nearStore, order string) (func(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64, cap int) ([]PlanningApplication, error), bool) {
	switch strings.TrimSpace(order) {
	case "", orderRecency:
		return store.RecentNearby, true
	case orderDistance:
		return store.NearestNearby, true
	default:
		return nil, false
	}
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
