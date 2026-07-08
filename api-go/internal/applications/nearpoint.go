package applications

import (
	"context"
	"encoding/base64"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// nearPointDefaultRadiusMetres, nearPointMinRadiusMetres, and
// nearPointMaxRadiusMetres bound the optional ?radius= query parameter: a
// present-but-out-of-range numeric value is CLAMPED into [min, max] rather than
// rejected (an omitted or genuinely unparseable value falls back to the
// default) — see parseNearPointRadius. This is a public, unauthenticated
// endpoint and, unlike the text search, a point+radius read is a whole-table
// scraping target (tile the UK with radius queries and dump the whole table):
// the radius/limit clamps here are a second, complementary layer of defense on
// top of the per-IP anonymous rate limiter (GH#868 Phase 1), not a substitute
// for it (GH#868 Phase 2).
const (
	nearPointDefaultRadiusMetres = 2000
	nearPointMinRadiusMetres     = 100
	nearPointMaxRadiusMetres     = 5000
)

// nearPointDefaultLimit and nearPointMaxLimit bound how many rows a single page
// returns; a present-but-out-of-range numeric ?limit= is clamped into
// [1, nearPointMaxLimit] (an omitted or unparseable value falls back to the
// default), mirroring the radius clamp above.
const (
	nearPointDefaultLimit = 100
	nearPointMaxLimit     = 200
)

// nearPointTimeout bounds a single near-point call end-to-end, matching
// search.go's searchTimeout: this is a public, unauthenticated endpoint
// sharing a Postgres pool with prod's core watch-zone/notification reads
// (psql-town-crier-shared) — a pathological query must fail fast with a 500
// rather than hold a pool connection open for tens of seconds (tc-z5i5j
// incident precedent). It is defense-in-depth alongside the radius/limit
// clamps above, not a substitute for them.
const nearPointTimeout = 8 * time.Second

// nearPointStore is the consumer-side store the near-point handler needs: the
// same generic KNN + ST_DWithin keyset page already used by the authed
// watch-zone nearby endpoints and the anonymous demo account. *PostgresStore
// satisfies it structurally.
type nearPointStore interface {
	FindNearbyPage(ctx context.Context, latitude, longitude, radiusMetres float64, limit int, cursor string) ([]PlanningApplication, string, error)
}

// nearPointHandler serves the public applications-near-a-point endpoint.
type nearPointHandler struct {
	store  nearPointStore
	logger *slog.Logger
}

// NearPointRoutes registers the public GET /v1/applications/near-point
// endpoint (GH#868 Phase 2). It is kept in cmd/api/wiring.go's
// anonymousPatterns — a DIFFERENT route from the build-key-gated
// GET /v1/applications/near SEO route (applications.NearRoutes) — and reads
// only public planning data from Postgres.
func NearPointRoutes(mux *http.ServeMux, store nearPointStore, logger *slog.Logger) {
	h := &nearPointHandler{store: store, logger: logger}
	mux.HandleFunc("GET /v1/applications/near-point", h.nearPoint)
}

// nearPoint validates lat/lng (required), radius/limit (optional, clamped),
// and cursor (optional, opaque), then returns one nearest-first page of
// applications within radius of (lat, lng). A missing/invalid lat/lng or a
// malformed cursor is a bodyless 400; a store failure is a bodyless 500 (both
// envelopes backfilled by middleware.ErrorBody). The response body is a bare
// JSON array of NearbyResult, with the continuation token (when present) in
// the X-Next-Cursor header — exact parity with the authed nearby list.
func (h *nearPointHandler) nearPoint(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	lat, lng, ok := parseNearPointCoordinates(q.Get("lat"), q.Get("lng"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	radius := parseNearPointRadius(q.Get("radius"))
	limit := parseNearPointLimit(q.Get("limit"))

	// decodeNearPointCursor strips the transport-layer base64 wrapping; a
	// malformed wrapper is a clean 400, never a silent reset to page one.
	cursor, ok := decodeNearPointCursor(q.Get("cursor"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), nearPointTimeout)
	defer cancel()

	apps, nextCursor, err := h.store.FindNearbyPage(ctx, lat, lng, radius, limit, cursor)
	if err != nil {
		serverError(w, r, h.logger, "find applications near point", err)
		return
	}

	results := make([]NearbyResult, 0, len(apps))
	for _, a := range apps {
		results = append(results, NearbyResultOf(a))
	}

	// Set the continuation header before writeJSON, which calls WriteHeader;
	// omitted entirely when the page is the last.
	if nextCursor != "" {
		w.Header().Set("X-Next-Cursor", encodeNearPointCursor(nextCursor))
	}
	writeJSON(w, r, h.logger, results)
}

// parseNearPointCoordinates parses and validates the required lat/lng query
// params: both must parse as finite floats in valid geographic range
// ([-90,90] / [-180,180]), mirroring watchzones.createRequest.valid(). Note
// strconv.ParseFloat accepts the literal strings "NaN"/"Inf"/"+Inf"/"-Inf", so
// the finiteness check is required even though ParseFloat itself succeeded.
func parseNearPointCoordinates(rawLat, rawLng string) (lat, lng float64, ok bool) {
	lat, err := strconv.ParseFloat(strings.TrimSpace(rawLat), 64)
	if err != nil {
		return 0, 0, false
	}
	lng, err = strconv.ParseFloat(strings.TrimSpace(rawLng), 64)
	if err != nil {
		return 0, 0, false
	}
	if math.IsNaN(lat) || math.IsInf(lat, 0) || math.IsNaN(lng) || math.IsInf(lng, 0) {
		return 0, 0, false
	}
	if lat < -90 || lat > 90 {
		return 0, 0, false
	}
	if lng < -180 || lng > 180 {
		return 0, 0, false
	}
	return lat, lng, true
}

// parseNearPointRadius clamps the optional ?radius= into
// [nearPointMinRadiusMetres, nearPointMaxRadiusMetres]. Unset, empty,
// non-numeric, or non-finite falls back to nearPointDefaultRadiusMetres; any
// other parseable value (including zero or negative) is clamped into the
// bound rather than rejected.
func parseNearPointRadius(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nearPointDefaultRadiusMetres
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return nearPointDefaultRadiusMetres
	}
	if n < nearPointMinRadiusMetres {
		return nearPointMinRadiusMetres
	}
	if n > nearPointMaxRadiusMetres {
		return nearPointMaxRadiusMetres
	}
	return n
}

// parseNearPointLimit clamps the optional ?limit= into [1, nearPointMaxLimit].
// Unset, empty, or non-numeric falls back to nearPointDefaultLimit; any other
// parseable value (including zero or negative) is clamped into the bound
// rather than rejected.
func parseNearPointLimit(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nearPointDefaultLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return nearPointDefaultLimit
	}
	if n < 1 {
		return 1
	}
	if n > nearPointMaxLimit {
		return nearPointMaxLimit
	}
	return n
}

// decodeNearPointCursor base64url-decodes an opaque ?cursor= value into the
// store's raw keyset continuation token, mirroring
// watchzones.decodeCursor/encodeCursor's exact semantics (that package is
// unexported, so this is a deliberate parallel implementation, not a shared
// one). An empty value means the first page (ok). A malformed value is
// rejected (ok == false) so a garbage cursor is a clean 400, not a silent
// reset to the first page.
func decodeNearPointCursor(raw string) (string, bool) {
	if raw == "" {
		return "", true
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// encodeNearPointCursor base64url-encodes the store's raw keyset continuation
// token for the X-Next-Cursor response header — header- and URL-safe, unpadded.
func encodeNearPointCursor(token string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(token))
}
