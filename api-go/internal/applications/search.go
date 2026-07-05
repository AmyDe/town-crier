package applications

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/AmyDe/town-crier/api-go/internal/authorities"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// searchMinQueryLen and searchMaxQueryLen bound the free-text q parameter: below
// the minimum q is rejected unless looksLikeReference reports it is probably a
// planning reference rather than an address/description fragment (a defensible,
// low-risk heuristic — see looksLikeReference's doc comment); above the maximum
// it is rejected outright, so a single request cannot force an expensive
// similarity/ts_rank scan with a pathological input size.
const (
	searchMinQueryLen = 3
	searchMaxQueryLen = 200
)

// searchDefaultLimit and searchMaxLimit bound how many rows the endpoint
// returns. Unlike the SEO endpoints (recentDefaultLimit/recentMaxLimit), v1 has
// no cursor: an over-limit match set is truncated and flagged via
// SearchResponse.RefineQuery rather than paginated.
const (
	searchDefaultLimit = 20
	searchMaxLimit     = 20
)

// searchStore is the consumer-side store the search handler needs: a single
// ranked, capped read across the three match tiers (#821 Phase 3) — reference
// exact/prefix match on uid, address fuzzy match (pg_trgm), description
// full-text match (tsvector). authorityCode "" means no authority filter (a
// bare reference legitimately matches across authorities — application_uid is
// only unique within a council). The bool return reports whether more matches
// exist beyond limit (RefineQuery on the wire); the concrete *PostgresStore
// satisfies it structurally.
type searchStore interface {
	Search(ctx context.Context, query, authorityCode string, limit int) ([]PlanningApplication, bool, error)
}

// searchHandler serves the anonymous application search endpoint.
type searchHandler struct {
	store    searchStore
	resolver authoritySlugResolver
	logger   *slog.Logger
}

// SearchRoutes registers the anonymous application search endpoint. It is kept
// out of auth's fallback-deny set in cmd/api/wiring.go's anonymousPatterns (like
// the by-slug application read) and reads only public planning data from
// Postgres — PlanIt is never touched by this endpoint (GH#395 Invariant 1).
func SearchRoutes(mux *http.ServeMux, store searchStore, resolver authoritySlugResolver, logger *slog.Logger) {
	h := &searchHandler{store: store, resolver: resolver, logger: logger}
	mux.HandleFunc("GET /v1/applications/search", h.search)
}

// search validates q (and the optional authority filter + limit), then returns
// the ranked match set: reference exact/prefix on uid, ranked above address
// fuzzy match, ranked above description full-text match. A missing/too-short q
// (unless it looks like a reference) or an unresolvable authority slug is a
// bodyless 400; a store failure is a bodyless 500 (both envelopes backfilled by
// middleware.ErrorBody).
func (h *searchHandler) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	query := strings.TrimSpace(q.Get("q"))
	if !validSearchQuery(query) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	authorityCode := ""
	if raw := strings.TrimSpace(q.Get("authority")); raw != "" {
		code, ok := resolveAuthorityParam(raw, h.resolver)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		authorityCode = code
	}

	limit := parseSearchLimit(q.Get("limit"))

	apps, refine, err := h.store.Search(r.Context(), query, authorityCode, limit)
	if err != nil {
		serverError(w, r, h.logger, "search applications", err)
		return
	}

	results := make([]SearchResult, 0, len(apps))
	for _, a := range apps {
		results = append(results, SearchResultOf(a, h.authoritySlug(r.Context(), a)))
	}

	writeJSON(w, r, h.logger, SearchResponse{
		Query:       query,
		Results:     results,
		RefineQuery: refine,
	})
}

// authoritySlug resolves the URL slug for the application's authority, mirroring
// (h *handler).authoritySlug in handler.go: it prefers the resolver's
// SlugForAreaID and falls back, with a warn log, to slugifying the raw PlanIt
// area name when the id is unknown to the static authorities data.
func (h *searchHandler) authoritySlug(ctx context.Context, app PlanningApplication) string {
	if slug, ok := h.resolver.SlugForAreaID(app.AreaID); ok {
		return slug
	}
	h.logger.WarnContext(ctx, "authority slug fallback: area id not in static authorities", "op", "search authority slug", "areaId", app.AreaID, "areaName", app.AreaName, "uid", app.UID)
	return authorities.Slugify(app.AreaName)
}

// validSearchQuery reports whether q is non-empty, no longer than
// searchMaxQueryLen, and either at least searchMinQueryLen runes or
// looksLikeReference.
func validSearchQuery(q string) bool {
	n := len([]rune(q))
	if n == 0 || n > searchMaxQueryLen {
		return false
	}
	if n >= searchMinQueryLen {
		return true
	}
	return looksLikeReference(q)
}

// looksLikeReference reports whether q contains at least one digit — the cheap
// heuristic for "this short query is probably a planning reference, not an
// address/description fragment" (tc-geq7h.3 decision 2026-07-05). Real
// references vary too widely in shape (24/0001/FUL, 9/P/2026/0044/HH,
// 24/SAME/FUL) to anchor a stricter pattern; over-admitting costs nothing since
// the reference tier is an indexed exact/prefix lookup on uid that simply
// returns zero rows on a miss.
func looksLikeReference(q string) bool {
	return strings.ContainsFunc(q, func(r rune) bool { return r >= '0' && r <= '9' })
}

// parseSearchLimit clamps the limit query parameter to [1, searchMaxLimit],
// defaulting to searchDefaultLimit when unset, empty, non-numeric, or
// non-positive.
func parseSearchLimit(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return searchDefaultLimit
	}
	if n > searchMaxLimit {
		return searchMaxLimit
	}
	return n
}

// resolveAuthorityParam resolves the authority=<id-or-slug> filter to an
// authority_code (the stringified area id): a numeric value is used directly
// (matching RecentRoutes/NearRoutes, which never validate a numeric id against
// the static authorities data either — an id with no applications is just an
// empty result, not an error); a non-numeric value is resolved as a slug via
// resolver.SlugToAreaID, and an unresolvable slug is reported false (-> 400),
// since a slug that cannot resolve at all is unambiguously a client error.
func resolveAuthorityParam(raw string, resolver authoritySlugResolver) (string, bool) {
	if id, err := strconv.Atoi(raw); err == nil {
		return strconv.Itoa(id), true
	}
	areaID, ok := resolver.SlugToAreaID(raw)
	if !ok {
		return "", false
	}
	return strconv.Itoa(areaID), true
}

// SearchResult is one ranked row of GET /v1/applications/search: just enough to
// render a result card and build a share URL client-side
// (share.towncrierapp.uk/a/{authoritySlug}/{reference}). Reference is
// planit_name (PlanningApplication.Name), NOT uid: uid is only ever the tier-1
// ranking's match key, never emitted, since the share route resolves by
// planit_name (sharepage/handler.go -> GetByAuthorityAndName) — echoing uid
// would 404 the link whenever uid != planit_name, which per real PlanIt data is
// the common case, not an edge case.
type SearchResult struct {
	Reference     string             `json:"reference"`
	AuthoritySlug string             `json:"authoritySlug"`
	AuthorityName string             `json:"authorityName"`
	Address       string             `json:"address"`
	AppState      *string            `json:"appState"`
	StartDate     *platform.DateOnly `json:"startDate"`
	DecidedDate   *platform.DateOnly `json:"decidedDate"`
}

// SearchResultOf maps a matched domain snapshot plus its resolved authority slug
// to the wire shape.
func SearchResultOf(a PlanningApplication, authoritySlug string) SearchResult {
	return SearchResult{
		Reference:     a.Name,
		AuthoritySlug: authoritySlug,
		AuthorityName: a.AreaName,
		Address:       a.Address,
		AppState:      a.AppState,
		StartDate:     platform.DateOnlyPtr(a.StartDate),
		DecidedDate:   platform.DateOnlyPtr(a.DecidedDate),
	}
}

// SearchResponse is the full wire body of GET /v1/applications/search. Results
// is always a non-null array (bounded at the request's limit, capped at
// searchMaxLimit). RefineQuery is true when the match set exceeds the limit: v1
// has no cursor, so rather than silently truncating a possibly much larger
// result set, the caller is told to narrow q/authority instead of paging.
type SearchResponse struct {
	Query       string         `json:"query"`
	Results     []SearchResult `json:"results"`
	RefineQuery bool           `json:"refineQuery"`
}
