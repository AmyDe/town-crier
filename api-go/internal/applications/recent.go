package applications

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// recentReadCap is the hard upper bound on documents read per authority in one
// SEO build request for the rendered list. The list query is bounded by it
// (SELECT TOP @cap), so the RU cost and response size of the card list stay flat
// regardless of how busy the authority is. It bounds only the rendered list, not
// the status breakdown — that is an index-served GROUP BY over the whole
// partition (BreakdownByAuthority).
const recentReadCap = 200

// recentDefaultLimit and recentMaxLimit bound how many applications the response
// renders. limit defaults to recentDefaultLimit and is hard-capped at
// recentMaxLimit; the rendered slice is clamped against the bounded read.
const (
	recentDefaultLimit = 30
	recentMaxLimit     = 100
)

// recentStore is the consumer-side store the SEO read handler needs: a bounded,
// single-partition top-N read by authority plus a whole-partition per-appState
// breakdown (whose buckets sum to the exact partition total). The concrete
// *CosmosStore satisfies it structurally.
type recentStore interface {
	RecentByAuthority(ctx context.Context, authorityCode string, cap int) ([]PlanningApplication, error)
	BreakdownByAuthority(ctx context.Context, authorityCode string) ([]StateCount, error)
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

	// breakdown is the per-appState distribution over the WHOLE partition (an
	// index-served GROUP BY), independent of the bounded read which saturates at
	// recentReadCap. The render slice must clamp against the bounded read length,
	// NOT the breakdown total — the total can dwarf len(apps).
	breakdown, err := h.store.BreakdownByAuthority(r.Context(), authorityCode)
	if err != nil {
		serverError(w, r, h.logger, "status breakdown by authority", err)
		return
	}

	// StatusBreakdown is always a non-null array on the wire; an empty partition
	// yields an empty (not nil) slice so it marshals to [] rather than null.
	if breakdown == nil {
		breakdown = []StateCount{}
	}

	// total is the EXACT whole-partition total: the sum of the breakdown buckets,
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

	// areaName is the same across the partition; take it from the first row when
	// present. With zero applications it is empty (the prerender gate skips such
	// authorities, so an empty page title never ships).
	areaName := ""
	if len(apps) > 0 {
		areaName = apps[0].AreaName
	}

	writeJSON(w, r, h.logger, RecentByAuthorityResult{
		AuthorityID:     id,
		AreaName:        areaName,
		Applications:    results,
		Total:           total,
		StatusBreakdown: breakdown,
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
