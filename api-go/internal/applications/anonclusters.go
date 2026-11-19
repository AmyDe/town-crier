package applications

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// clustersTimeout bounds a single anonymous clusters call end-to-end, for the
// same reason as nearPointTimeout.
const clustersTimeout = 8 * time.Second

type clustersStore interface {
	FindClustersInZone(ctx context.Context, q ClusterQuery) ([]Cluster, error)
}

type clustersHandler struct {
	store    clustersStore
	resolver authoritySlugResolver
	logger   *slog.Logger
}

// ClustersRoutes registers the public GET /v1/applications/clusters endpoint,
// serving the same grid-aggregated clusters as the authed watch-zone map.
func ClustersRoutes(mux *http.ServeMux, store clustersStore, resolver authoritySlugResolver, logger *slog.Logger) {
	h := &clustersHandler{store: store, resolver: resolver, logger: logger}
	mux.HandleFunc("GET /v1/applications/clusters", h.clusters)
}

// clusters returns the grid-aggregated clusters for the caller-supplied
// centre, radius and viewport as a bare JSON array (never null), each member
// identity enriched with its AuthoritySlug. Invalid lat/lng, bbox, zoom or
// status is a bodyless 400 and a store failure a bodyless 500.
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
		// Always the finest grid cell size, independent of the request's zoom.
		CoalesceThresholdDegrees: FinestGridDegrees(),
	})
	if err != nil {
		serverError(w, r, h.logger, "find clusters near point", err)
		return
	}

	if clusters == nil {
		clusters = []Cluster{}
	}
	h.enrichAuthoritySlugs(r.Context(), clusters)
	writeJSON(w, r, h.logger, clusters)
}

// parseClustersZoom rejects rather than clamps: zoom has no sensible default.
func parseClustersZoom(raw string) (float64, bool) {
	z, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return GridDegreesForZoom(z)
}

// parseClustersStatus mirrors watchzones' parseStatus: "All" or absent means
// no filter.
func parseClustersStatus(raw string) (string, bool) {
	if raw == "" || raw == "All" {
		return "", true
	}
	if StatusSupported(raw) {
		return raw, true
	}
	return "", false
}

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

// attachAuthoritySlug treats a malformed authority or an area id missing from
// the static authorities table as a warn-logged miss that leaves the slug
// empty; unlike resolveAuthoritySlugFor there is no AreaName to slugify.
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
