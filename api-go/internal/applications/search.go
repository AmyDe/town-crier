package applications

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

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

// searchTimeout bounds a single Search call end-to-end. This is a public,
// unauthenticated endpoint (SearchRoutes) sharing a Postgres pool with prod's
// core watch-zone/notification reads (psql-town-crier-shared) — a
// pathological query must fail fast with a 500 rather than hold a pool
// connection open for tens of seconds (tc-z5i5j incident: ?q=extension took
// 15-53s before searchQuery's per-tier LIMIT fix). It is defense-in-depth on
// top of that fix, not a substitute for it, and is well under the ~100s
// Cloudflare/ingress timeout so callers see a clean failure rather than a
// gateway timeout.
const searchTimeout = 8 * time.Second

// searchStore is the consumer-side store the search handler needs: a
// nearest-first, capped read across the three match tiers (GH#863 API
// section) — reference exact/prefix match on uid, address fuzzy match
// (pg_trgm), description full-text match (tsvector) — merged, deduplicated by
// identity, and ordered by distance from (lat, lon) ascending. authorityCode
// "" means no authority filter (a bare reference legitimately matches across
// authorities — application_uid is only unique within a council). The bool
// return reports whether more matches exist beyond limit (RefineQuery on the
// wire); the concrete *PostgresStore satisfies it structurally.
type searchStore interface {
	Search(ctx context.Context, query, authorityCode string, lat, lon float64, limit int) ([]PlanningApplication, bool, error)
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

// search validates q, the mandatory lat/lon query point, and the optional
// authority filter + limit, then returns the nearest-first match set: the
// union of the three match tiers (reference exact/prefix on uid, address
// fuzzy match, description full-text match), deduplicated by identity and
// ordered by distance from (lat, lon) ascending (GH#863 API section — KNN
// nearest-first, mandatory location). A missing/too-short q (unless it looks
// like a reference), a missing/unparseable lat or lon, or an unresolvable
// authority slug is a bodyless 400; a store failure is a bodyless 500 (both
// envelopes backfilled by middleware.ErrorBody).
func (h *searchHandler) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	query := strings.TrimSpace(q.Get("q"))
	if !validSearchQuery(query) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	lat, ok := parseRequiredCoord(q.Get("lat"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	lon, ok := parseRequiredCoord(q.Get("lon"))
	if !ok {
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

	ctx, cancel := context.WithTimeout(r.Context(), searchTimeout)
	defer cancel()

	apps, refine, err := h.store.Search(ctx, query, authorityCode, lat, lon, limit)
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

// authoritySlug returns the URL slug for the application's authority. See
// resolveAuthoritySlugFor (respond.go) for the round-trip/fallback behaviour.
func (h *searchHandler) authoritySlug(ctx context.Context, app PlanningApplication) string {
	return resolveAuthoritySlugFor(ctx, h.resolver, h.logger, "search authority slug", app)
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

// parseRequiredCoord parses a mandatory lat/lon query parameter as a float64,
// reporting false for a missing, empty, or unparseable value (-> bodyless 400
// via search's callers). lat/lon are mandatory as of GH#863 — the KNN
// nearest-first redesign has no meaning without a query point — unlike
// validSearchQuery's q or resolveAuthorityParam's optional authority filter,
// so this has no "absent means default" branch.
func parseRequiredCoord(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return v, true
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

// searchPoint is the KNN query point for the three search tiers, built from
// $1 (longitude) and $2 (latitude) — matching nearbyPoint's (store_postgres.go)
// $-numbering convention for every other authority-agnostic spatial read.
const searchPoint = "ST_SetSRID(ST_MakePoint($1, $2), 4326)::geography"

// searchTier1Query, searchTier2Query, searchTier3Query each answer one match
// tier of GET /v1/applications/search (GH#863 API section — mandatory
// lat/lon, KNN nearest-first): reference exact/prefix match on uid, address
// pg_trgm fuzzy match, description tsvector full-text match. Each is shaped
// exactly like findNearbyFirstPageQuery (store_postgres.go) — appColumns plus
// a computed "location <-> point AS distance", ordered by that KNN distance
// then planit_name, capped at $6 — but with the tier's own text-match WHERE
// predicate in place of findNearbyFirstPageQuery's ST_DWithin, and NO radius
// bound: a nearest-first search has no reason to exclude a genuine match just
// because it is far away, unlike the radius-bounded browse endpoints.
//
// Search runs the three queries separately (not unioned) and merges their
// results in Go, deduplicating by identity and re-sorting by distance — a
// deliberate departure from the pre-GH#863 shape, which unioned all three
// tiers into one statement and ranked by a fixed tier priority (reference >
// address > description, tc-z5i5j) rather than distance.
//
// Each query independently asks for the caller's requested limit+1 rows
// (nearest-first, planit_name tie-break) via $6. This mirrors the old
// per-branch cap's correctness argument, re-derived for distance instead of
// tier/score: every tier's row set is a subset of the true global
// distance-ordered candidate set S (an application matching tier T is, by
// definition, a member of S), so an application's RANK within tier T's own
// distance ordering can never exceed its rank within the global order of S.
// Concretely: if application A is among the globally-nearest limit+1
// distinct matches, then within ANY tier T it matches, at most limit other
// tier-T matches can be nearer than A (each of those would also need to be
// globally nearer than A, and A is already at global rank <= limit+1) — so A
// is guaranteed to be within tier T's own top limit+1 by distance, and this
// per-tier cap captures it. Capping at limit+1, not limit, is what lets
// Search still detect "more matches exist" after the merge/dedup (the
// RefineQuery signal) exactly as searchQuery's old limit+1 did.
const searchTier1Query = "SELECT " + appColumns + ", location <-> " + searchPoint + " AS distance " +
	"FROM applications WHERE (lower(uid) = lower($3) OR lower(uid) LIKE $4 ESCAPE '\\') " +
	"AND ($5::text IS NULL OR authority_code = $5) " +
	"ORDER BY location <-> " + searchPoint + ", planit_name LIMIT $6"

const searchTier2Query = "SELECT " + appColumns + ", location <-> " + searchPoint + " AS distance " +
	"FROM applications WHERE lower($3) <% lower(address) " +
	"AND ($4::text IS NULL OR authority_code = $4) " +
	"ORDER BY location <-> " + searchPoint + ", planit_name LIMIT $5"

const searchTier3Query = "SELECT " + appColumns + ", location <-> " + searchPoint + " AS distance " +
	"FROM applications WHERE to_tsvector('english', description) @@ plainto_tsquery('english', $3) " +
	"AND ($4::text IS NULL OR authority_code = $4) " +
	"ORDER BY location <-> " + searchPoint + ", planit_name LIMIT $5"

// escapeLikeWildcards backslash-escapes the LIKE metacharacters (\, %, _) in a
// user-supplied query before it is used to build a prefix pattern, so a query
// containing a literal '%' or '_' cannot silently widen the tier-1 prefix match
// beyond an exact prefix of the typed text.
func escapeLikeWildcards(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(s)
}

// appIdentity is the natural-key dedup key for merging the three tiers'
// results: an application matching more than one tier (e.g. its address AND
// its description both mention the query) must surface exactly once, keeping
// whichever tier's copy is found first — the identity is what matters, not
// which tier matched, since GH#863 dropped tier-priority ranking in favour of
// pure distance ordering.
type appIdentity struct {
	areaID int
	name   string
}

// Search runs the three match tiers' KNN queries (searchTier1Query,
// searchTier2Query, searchTier3Query — sequentially, not concurrently, a
// deliberate GH#863 scope decision) against the mandatory (lat, lon) query
// point, merges their rows deduplicated by (area_id, planit_name), sorts the
// deduplicated set by distance ascending then planit_name ascending, and
// reports whether more matches exist beyond limit: each tier query already
// asks for limit+1 rows (see the tier query doc comment for why that cap is
// still correct under distance-first merging), so if the deduplicated,
// re-sorted set has more than limit rows, Search truncates to limit and
// returns true (the v1 "no cursor — tell the caller to refine" signal,
// SearchResponse.RefineQuery). authorityCode "" applies no authority filter,
// passed to each query as a genuine SQL NULL (not empty-string equality) so
// "($n::text IS NULL OR ...)" reads naturally.
func (s *PostgresStore) Search(ctx context.Context, query, authorityCode string, lat, lon float64, limit int) ([]PlanningApplication, bool, error) {
	var authorityArg any
	if authorityCode != "" {
		authorityArg = authorityCode
	}
	likePattern := strings.ToLower(escapeLikeWildcards(query)) + "%"
	perTierCap := limit + 1

	tiers := []struct {
		sql  string
		args []any
	}{
		{searchTier1Query, []any{lon, lat, query, likePattern, authorityArg, perTierCap}},
		{searchTier2Query, []any{lon, lat, query, authorityArg, perTierCap}},
		{searchTier3Query, []any{lon, lat, query, authorityArg, perTierCap}},
	}

	merged := make(map[appIdentity]nearbyRow)
	for i, t := range tiers {
		rows, err := s.db.Query(ctx, t.sql, t.args...)
		if err != nil {
			return nil, false, fmt.Errorf("search applications %q (tier %d): %w", query, i+1, err)
		}
		tierRows, err := pgx.CollectRows(rows, scanNearbyRow)
		if err != nil {
			return nil, false, fmt.Errorf("search applications %q (tier %d): %w", query, i+1, err)
		}
		for _, r := range tierRows {
			id := appIdentity{areaID: r.app.AreaID, name: r.app.Name}
			if _, exists := merged[id]; !exists {
				merged[id] = r
			}
		}
	}

	deduped := make([]nearbyRow, 0, len(merged))
	for _, r := range merged {
		deduped = append(deduped, r)
	}
	sort.Slice(deduped, func(i, j int) bool {
		if deduped[i].dist != deduped[j].dist {
			return deduped[i].dist < deduped[j].dist
		}
		return deduped[i].app.Name < deduped[j].app.Name
	})

	refine := len(deduped) > limit
	if refine {
		deduped = deduped[:limit]
	}

	apps := make([]PlanningApplication, len(deduped))
	for i, r := range deduped {
		apps[i] = r.app
	}
	return apps, refine, nil
}
