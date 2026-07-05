package applications

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

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

// searchQuery ranks applications matching q across three tiers in a single
// statement (#821 Phase 3), each computed once over the whole table and unioned:
//
//  1. Reference exact/prefix match on uid (case-insensitive): an exact match
//     scores 2.0, a prefix-only match 1.0, so within tier 1 an exact hit still
//     ranks first. uid — NOT planit_name — is the fuller, human-recognisable
//     council reference (tc-geq7h.3 decision 2026-07-05); it may legitimately
//     match rows in more than one authority, since a bare reference is only
//     unique within a council.
//  2. Address fuzzy match via pg_trgm word_similarity (<%): the query is
//     typically a short fragment of a much longer address, so word_similarity
//     (matching against any word-boundary substring of address) is used
//     instead of whole-string similarity (%), which would under-score a short,
//     otherwise-exact fragment.
//  3. Description full-text match via the english tsvector config, ranked by
//     ts_rank.
//
// matched unions the three tiers (each row tagged with its tier and an
// in-tier score); best keeps only the single best-tier match per application
// (DISTINCT ON (area_id, planit_name), the natural-key equivalent already
// present in appColumns) — an application matching more than one tier is never
// duplicated, it surfaces once under its highest-priority tier. The final
// SELECT re-applies the tier/score order (a plain column reference survives
// DISTINCT ON's row selection but not its projection, so the ORDER BY must be
// restated) plus a planit_name tie-break for determinism, and the LIMIT is the
// caller's requested limit+1 — the extra row is how Search detects "more
// matches exist than the limit" without a second query.
//
// searchSelectColumns (not appColumns) backs the three UNION branches: it
// aliases the two computed geometry accessors to lat/lon. appColumns' raw form
// (ST_Y(location::geometry) with no alias) only resolves because those
// branches SELECT directly FROM applications, where the location column is in
// scope — but matched/best are CTEs, and a CTE's exposed columns are named
// from their SELECT list (an unaliased function call is named after the
// function, not "location"), so location does not exist there. The outer
// SELECT list (searchOutputColumns) therefore reads the already-computed lat/
// lon straight off best, rather than re-deriving them from a location column
// that was never carried through the CTE chain.
const searchSelectColumns = "planit_name, uid, area_name, area_id, address, postcode, " +
	"description, app_type, app_state, app_size, start_date, decided_date, " +
	"consulted_date, ST_Y(location::geometry) AS lat, ST_X(location::geometry) AS lon, url, link, last_different"

const searchOutputColumns = "planit_name, uid, area_name, area_id, address, postcode, " +
	"description, app_type, app_state, app_size, start_date, decided_date, " +
	"consulted_date, lat, lon, url, link, last_different"

const searchQuery = `
WITH matched AS (
	SELECT ` + searchSelectColumns + `, 1 AS tier,
	       CASE WHEN lower(uid) = lower($1) THEN 2.0 ELSE 1.0 END AS score
	FROM applications
	WHERE (lower(uid) = lower($1) OR lower(uid) LIKE $4 ESCAPE '\')
	  AND ($2::text IS NULL OR authority_code = $2)
	UNION ALL
	SELECT ` + searchSelectColumns + `, 2 AS tier, word_similarity(lower($1), lower(address)) AS score
	FROM applications
	WHERE lower($1) <% lower(address)
	  AND ($2::text IS NULL OR authority_code = $2)
	UNION ALL
	SELECT ` + searchSelectColumns + `, 3 AS tier,
	       ts_rank(to_tsvector('english', description), plainto_tsquery('english', $1)) AS score
	FROM applications
	WHERE to_tsvector('english', description) @@ plainto_tsquery('english', $1)
	  AND ($2::text IS NULL OR authority_code = $2)
),
best AS (
	SELECT DISTINCT ON (area_id, planit_name) *
	FROM matched
	ORDER BY area_id, planit_name, tier ASC, score DESC
)
SELECT ` + searchOutputColumns + `
FROM best
ORDER BY tier ASC, score DESC, planit_name ASC
LIMIT $3`

// escapeLikeWildcards backslash-escapes the LIKE metacharacters (\, %, _) in a
// user-supplied query before it is used to build a prefix pattern, so a query
// containing a literal '%' or '_' cannot silently widen the tier-1 prefix match
// beyond an exact prefix of the typed text.
func escapeLikeWildcards(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
}

// Search runs searchQuery and reports whether more matches exist beyond limit:
// it asks for limit+1 rows, and if that many come back, truncates to limit and
// returns true (the v1 "no cursor — tell the caller to refine" signal,
// SearchResponse.RefineQuery). authorityCode "" applies no authority filter,
// passed to the query as a genuine SQL NULL (not empty-string equality) so
// "($2::text IS NULL OR ...)" reads naturally.
func (s *PostgresStore) Search(ctx context.Context, query, authorityCode string, limit int) ([]PlanningApplication, bool, error) {
	var authorityArg any
	if authorityCode != "" {
		authorityArg = authorityCode
	}
	likePattern := strings.ToLower(escapeLikeWildcards(query)) + "%"

	rows, err := s.db.Query(ctx, searchQuery, query, authorityArg, limit+1, likePattern)
	if err != nil {
		return nil, false, fmt.Errorf("search applications %q: %w", query, err)
	}
	apps, err := pgx.CollectRows(rows, scanAppRow)
	if err != nil {
		return nil, false, fmt.Errorf("search applications %q: %w", query, err)
	}

	refine := len(apps) > limit
	if refine {
		apps = apps[:limit]
	}
	return apps, refine, nil
}
