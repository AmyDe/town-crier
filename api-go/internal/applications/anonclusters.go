package applications

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// clustersTimeout bounds a single anonymous clusters call end-to-end, mirroring
// nearPointTimeout (same rationale: a public, unauthenticated endpoint sharing
// a Postgres pool with prod's core watch-zone/notification reads must fail
// fast with a 500 rather than hold a pool connection open indefinitely —
// tc-z5i5j precedent; also relevant given tc-ai55's bimodal clusters latency).
// It is defense-in-depth alongside the radius clamp, not a substitute for it.
const clustersTimeout = 8 * time.Second

// clustersStore is the consumer-side store the anonymous clusters handler
// needs: the same grid-aggregation primitive the authed watch-zone clusters
// endpoint uses, taking a caller-supplied centre/radius instead of a stored
// zone. *PostgresStore satisfies it structurally.
type clustersStore interface {
	FindClustersInZone(ctx context.Context, q ClusterQuery) ([]Cluster, error)
}

// clustersHandler serves the public anonymous clusters endpoint.
type clustersHandler struct {
	store    clustersStore
	resolver authoritySlugResolver
	logger   *slog.Logger
}

// ClustersRoutes registers the public GET /v1/applications/clusters endpoint
// (GH#924 Phase 1). It backs the iOS anonymous browse map with the same
// PostGIS grid-aggregated clusters the authed watch-zone map renders
// (FindClustersInZone, issue #698), so an anonymous visitor sees the fully
// clustered set across their radius circle instead of a client-side cluster
// over a truncated near-point page. It is kept in cmd/api/wiring.go's
// anonymousPatterns, mirroring NearPointRoutes.
func ClustersRoutes(mux *http.ServeMux, store clustersStore, resolver authoritySlugResolver, logger *slog.Logger) {
	h := &clustersHandler{store: store, resolver: resolver, logger: logger}
	mux.HandleFunc("GET /v1/applications/clusters", h.clusters)
}

// clusters validates lat/lng (required), bbox (required), zoom (required, 0..
// maxZoom), radius (optional, clamped like near-point) and status (optional,
// same vocabulary as the authed zone-clusters endpoint), then returns the
// grid-aggregated clusters for the caller-supplied centre/radius/viewport as a
// bare JSON array (never null). A missing/invalid lat/lng, bbox, zoom, or
// status is a bodyless 400; a store failure is a bodyless 500 (both envelopes
// backfilled by middleware.ErrorBody, mirroring nearPointHandler.nearPoint).
// Every result identity (Member and every Members entry) is enriched with its
// AuthoritySlug before encoding.
func (h *clustersHandler) clusters(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	lat, lng, ok := parseNearPointCoordinates(q.Get("lat"), q.Get("lng"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	box, ok := ParseBBox(q.Get("bbox"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	gridSize, ok := parseClustersZoom(q.Get("zoom"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	status, ok := parseClustersStatus(q.Get("status"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	radius := parseNearPointRadius(q.Get("radius"))

	ctx, cancel := context.WithTimeout(r.Context(), clustersTimeout)
	defer cancel()

	clusters, err := h.store.FindClustersInZone(ctx, ClusterQuery{
		Latitude:        lat,
		Longitude:       lng,
		RadiusMetres:    radius,
		West:            box.West,
		South:           box.South,
		East:            box.East,
		North:           box.North,
		GridSizeDegrees: gridSize,
		Status:          status,
		// The coalesce threshold is always the finest (zoom-20) grid cell size,
		// independent of the request's own zoom/grid — see FinestGridDegrees's
		// doc comment, mirroring watchzones' clusters handler.
		CoalesceThresholdDegrees: FinestGridDegrees(),
	})
	if err != nil {
		serverError(w, r, h.logger, "find clusters near point", err)
		return
	}

	// Encode a bare JSON array, never null, for an empty viewport.
	if clusters == nil {
		clusters = []Cluster{}
	}
	h.enrichAuthoritySlugs(r.Context(), clusters)
	writeJSON(w, r, h.logger, clusters)
}

// parseClustersZoom resolves the required ?zoom= to a grid cell size via
// GridDegreesForZoom. Unlike parseNearPointRadius (which clamps), zoom has no
// sensible default: an absent, non-integer, or out-of-range value is a clean
// 400, mirroring watchzones' parseZoom.
func parseClustersZoom(raw string) (float64, bool) {
	if raw == "" {
		return 0, false
	}
	z, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return GridDegreesForZoom(z)
}

// parseClustersStatus resolves the optional ?status= to an app_state filter,
// mirroring watchzones' parseStatus (same vocabulary; "All"/absent both mean
// no filter). The anonymous map does not send this param today (GH#924 keeps
// its reduced feature set — no filter chips) but the endpoint supports it for
// symmetry with the authed clusters endpoint.
func parseClustersStatus(raw string) (string, bool) {
	if raw == "" || raw == "All" {
		return "", true
	}
	if StatusSupported(raw) {
		return raw, true
	}
	return "", false
}

// enrichAuthoritySlugs populates AuthoritySlug on every member identity in
// clusters: a single-member cell's Member, and every entry of an unsplittable
// multi-member cell's Members disambiguation list.
func (h *clustersHandler) enrichAuthoritySlugs(ctx context.Context, clusters []Cluster) {
	for i := range clusters {
		if clusters[i].Member != nil {
			h.attachAuthoritySlug(ctx, clusters[i].Member)
		}
		for j := range clusters[i].Members {
			h.attachAuthoritySlug(ctx, &clusters[i].Members[j])
		}
	}
}

// attachAuthoritySlug resolves id.Authority (the area_id rendered as a decimal
// string, see PlanningApplicationID's doc comment) to its URL slug via
// resolver.SlugForAreaID and sets id.AuthoritySlug. A malformed authority
// string, or an area id absent from the static authorities table (should
// never happen — the table covers every real authority), is a warn-logged
// miss that leaves the slug empty, mirroring resolveAuthoritySlugFor's warn
// posture but with no AreaName to fall back to slugifying (a Cluster member
// carries none).
func (h *clustersHandler) attachAuthoritySlug(ctx context.Context, id *PlanningApplicationID) {
	areaID, err := strconv.Atoi(id.Authority)
	if err != nil {
		h.logger.WarnContext(ctx, "authority slug enrichment: authority is not a valid area id", "authority", id.Authority, "name", id.Name)
		return
	}
	slug, ok := h.resolver.SlugForAreaID(areaID)
	if !ok {
		h.logger.WarnContext(ctx, "authority slug enrichment: area id not in static authorities", "areaId", areaID, "name", id.Name)
		return
	}
	id.AuthoritySlug = slug
}
