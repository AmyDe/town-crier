package applications

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// recentReadCap is the hard upper bound on documents read per authority in one
// SEO build request. The query is bounded by it (SELECT TOP @cap), so the RU cost
// and response size stay flat regardless of how busy the authority is.
const recentReadCap = 200

// recentDefaultLimit and recentMaxLimit bound how many applications the response
// renders. limit defaults to recentDefaultLimit and is hard-capped at
// recentMaxLimit; the full bounded read still backs total/totalCapped.
const (
	recentDefaultLimit = 30
	recentMaxLimit     = 100
)

// recentStore is the consumer-side store the SEO read handler needs: a single
// bounded, single-partition top-N read by authority. The concrete *CosmosStore
// satisfies it structurally.
type recentStore interface {
	RecentByAuthority(ctx context.Context, authorityCode string, cap int) ([]PlanningApplication, error)
}

// recentHandler serves the build-time SEO endpoint.
type recentHandler struct {
	store  recentStore
	logger *slog.Logger
}

// RecentRoutes registers the build-key-gated recent-applications-by-authority
// endpoint. The route is anonymous to Auth0 (kept out of the fallback-deny set in
// wiring) and authenticated solely by the X-Build-Key gate, mirroring the admin
// surface. It exists for the static SEO prerender, which reads only public
// planning data from Cosmos.
func RecentRoutes(mux *http.ServeMux, store recentStore, buildKey string, logger *slog.Logger) {
	h := &recentHandler{store: store, logger: logger}
	mux.HandleFunc("GET /v1/authorities/{id}/applications", requireBuildKey(buildKey, h.recentByAuthority))
}

// recentByAuthority returns up to limit most-recently-active applications for the
// numeric authority id, drawn from one bounded single-partition read of at most
// recentReadCap documents. There is no PlanIt fallback (GH#395 Invariant 1).
func (h *recentHandler) recentByAuthority(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		// The build key was valid; a non-numeric authority id is a client error.
		// Bodyless 400 — the PascalCase envelope is backfilled downstream.
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	limit := parseLimit(r.URL.Query().Get("limit"))
	authorityCode := strconv.Itoa(id)

	apps, err := h.store.RecentByAuthority(r.Context(), authorityCode, recentReadCap)
	if err != nil {
		serverError(w, r, h.logger, "recent applications by authority", err)
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

	// areaName is the same across the partition; take it from the first row when
	// present. With zero applications it is empty (the prerender gate skips such
	// authorities, so an empty page title never ships).
	areaName := ""
	if total > 0 {
		areaName = apps[0].AreaName
	}

	writeJSON(w, r, h.logger, RecentByAuthorityResult{
		AuthorityID:  id,
		AreaName:     areaName,
		Applications: results,
		Total:        total,
		TotalCapped:  total >= recentReadCap,
	})
}

// parseLimit clamps the limit query parameter to [1, recentMaxLimit], defaulting
// to recentDefaultLimit when unset, empty, non-numeric, or non-positive.
func parseLimit(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return recentDefaultLimit
	}
	if n > recentMaxLimit {
		return recentMaxLimit
	}
	return n
}
