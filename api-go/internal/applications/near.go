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

// nearDefaultRadiusMetres and nearMaxRadiusMetres bound the spatial search radius
// — both the target town's own radius and each sibling centroid's own radius.
// radius defaults to nearDefaultRadiusMetres (a town-centroid catchment) when
// unset and is hard-clamped to nearMaxRadiusMetres, so a build request can never
// widen the partition scan beyond the intended town footprint.
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

// maxSiblingCentroids bounds how many sibling-town centroids a single request
// may pass, so the partition query's towns CTE can never fan out unboundedly.
// No authority's gazetteer approaches this many towns.
const maxSiblingCentroids = 64

// nearStore is the consumer-side store the town-level SEO handler needs: the
// bounded, authority-scoped Voronoi-partition read (RecentNearestTown, #819)
// plus a whole-in-radius per-appState breakdown over the target town's own
// point/radius (whose buckets sum to the exact in-radius total; it is NOT
// partitioned — see the field comment on RecentNearbyResult). The concrete
// *PostgresStore satisfies it structurally.
type nearStore interface {
	RecentNearestTown(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64, siblings []TownCentroid, cap int) ([]PlanningApplication, error)
	BreakdownNearby(ctx context.Context, authorityCode string, lat, lng, radiusMetres float64) ([]StateCount, error)
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
// planning data from Postgres (GH#395 Invariant 1 — never PlanIt).
func NearRoutes(mux *http.ServeMux, store nearStore, buildKey string, logger *slog.Logger) {
	h := &nearHandler{store: store, logger: logger}
	mux.HandleFunc("GET /v1/applications/near", requireBuildKey(buildKey, h.recentNearby))
}

// recentNearby returns up to limit applications assigned to THIS town by the
// query-time Voronoi partition (#819): among this town's own (lat, lng, radius)
// and the repeated "sibling" centroids (each an OTHER gazetteer town in the
// same authority, with its own radius), an application is kept only if at
// least one of them covers it, assigned to whichever covering town is nearest,
// and returned only when that town is THIS one — closest-wins, in-range-
// nearest, single assignment, no overlap between sibling town pages. Ordered
// by the real-date key (GREATEST(decidedDate, startDate) DESC NULLS LAST).
// authorityId scopes the read; a missing/invalid id, a non-finite or
// out-of-range coordinate, a non-finite/non-positive radius, a malformed or
// excessive sibling list, is a bodyless 400 (the build key was already
// validated by the gate). There is no PlanIt fallback.
func (h *nearHandler) recentNearby(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// authorityId is required: it scopes the spatial query to one authorityCode
	// (the AreaID as a string), exactly as the authority endpoint does.
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
	siblings, ok := parseSiblingCentroids(q["sibling"])
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	limit := parseLimit(q.Get("limit"))
	authorityCode := strconv.Itoa(id)

	apps, err := h.store.RecentNearestTown(r.Context(), authorityCode, lat, lng, radius, siblings, nearReadCap)
	if err != nil {
		serverError(w, r, h.logger, "recent applications nearest town", err)
		return
	}

	// breakdown is the per-appState distribution over the WHOLE in-radius set
	// around THIS town's own point (an index-served spatial GROUP BY) — it is
	// deliberately NOT partitioned by sibling centroids, independent of the
	// bounded, partitioned read which saturates at nearReadCap. The render slice
	// must clamp against the bounded read length, NOT the breakdown total — the
	// total can dwarf len(apps).
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

// clampRadius validates a required, finite, positive radius and clamps it down
// to nearMaxRadiusMetres, so no radius (the target town's own, or a sibling's)
// can widen a spatial scan beyond the hard maximum. Shared by parseRadius
// (which additionally defaults an empty value) and parseSiblingCentroids
// (whose radius component is always required, never defaulted).
func clampRadius(v float64) (float64, bool) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0, false
	}
	if v > nearMaxRadiusMetres {
		return nearMaxRadiusMetres, true
	}
	return v, true
}

// parseRadius parses the optional radius in metres: it defaults to
// nearDefaultRadiusMetres when unset/empty, rejects a non-numeric value (false
// -> 400), and otherwise validates/clamps via clampRadius.
func parseRadius(raw string) (float64, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nearDefaultRadiusMetres, true
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return clampRadius(v)
}

// parseSiblingCentroids parses the repeated "sibling" query values, each a
// "lat,lng,radius" triple identifying one OTHER gazetteer town's centroid (and
// its own safety radius) in the requested town's authority — the requested
// town's own point/radius are the primary lat/lng/radius params, not repeated
// here (#819 decisions 2-3). Every lat/lng is validated with the same WGS84
// bounds as the primary point; every radius is validated and clamped exactly
// like the primary radius (clampRadius). More than maxSiblingCentroids values,
// or any malformed/invalid entry, is a 400 (false). A nil/empty raw is valid
// and yields an empty (non-partitioned) slice.
func parseSiblingCentroids(raw []string) ([]TownCentroid, bool) {
	if len(raw) > maxSiblingCentroids {
		return nil, false
	}
	out := make([]TownCentroid, 0, len(raw))
	for _, v := range raw {
		parts := strings.Split(v, ",")
		if len(parts) != 3 {
			return nil, false
		}
		lat, ok := parseCoordinate(parts[0], minLatitude, maxLatitude)
		if !ok {
			return nil, false
		}
		lng, ok := parseCoordinate(parts[1], minLongitude, maxLongitude)
		if !ok {
			return nil, false
		}
		radiusVal, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		if err != nil {
			return nil, false
		}
		radius, ok := clampRadius(radiusVal)
		if !ok {
			return nil, false
		}
		out = append(out, TownCentroid{Lat: lat, Lng: lng, RadiusMetres: radius})
	}
	return out, true
}
