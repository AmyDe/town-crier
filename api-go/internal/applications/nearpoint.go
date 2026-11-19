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
// default). A public point+radius read is a whole-table scraping target, so
// these clamps are a second layer of defence behind the per-IP rate limiter.
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

// nearPointSortDistance and nearPointSortRecent are the accepted ?sort= values.
// nearPointSortDistance is the default nearest-first KNN order with keyset
// pagination via ?cursor=/X-Next-Cursor. nearPointSortRecent orders by
// RecentRealDateOrder (store_postgres.go) and does NOT paginate: a full page
// never sets X-Next-Cursor.
const (
	nearPointSortDistance = "distance"
	nearPointSortRecent   = "recent"
)

// nearPointTimeout bounds a single near-point call end-to-end, matching
// searchTimeout: a pathological query on this public endpoint must fail fast
// rather than hold a shared pool connection.
const nearPointTimeout = 8 * time.Second

// nearPointStore: FindNearbyPage serves ?sort=distance (the default);
// RecentNearPoint serves ?sort=recent.
type nearPointStore interface {
	FindNearbyPage(ctx context.Context, latitude, longitude, radiusMetres float64, limit int, cursor string) ([]PlanningApplication, string, error)
	RecentNearPoint(ctx context.Context, latitude, longitude, radiusMetres float64, limit int) ([]PlanningApplication, error)
}

type nearPointHandler struct {
	store    nearPointStore
	resolver authoritySlugResolver
	logger   *slog.Logger
}

// NearPointRoutes registers the public GET /v1/applications/near-point
// endpoint. The resolver populates each result's AuthoritySlug so an
// anonymously-loaded application can build a share URL or a by-slug detail
// fetch.
func NearPointRoutes(mux *http.ServeMux, store nearPointStore, resolver authoritySlugResolver, logger *slog.Logger) {
	h := &nearPointHandler{store: store, resolver: resolver, logger: logger}
	mux.HandleFunc("GET /v1/applications/near-point", h.nearPoint)
}

// nearPoint returns one page of applications within radius of (lat, lng) as a
// bare JSON array of NearbyResult. Invalid lat/lng, sort or cursor is a
// bodyless 400 and a store failure a bodyless 500 (envelopes backfilled by
// middleware.ErrorBody). Only the distance sort sets X-Next-Cursor.
func (h *nearPointHandler) nearPoint(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	lat, lng, ok := parseNearPointCoordinates(q.Get("lat"), q.Get("lng"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	sort, ok := parseNearPointSort(q.Get("sort"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	radius := parseNearPointRadius(q.Get("radius"))
	limit := parseNearPointLimit(q.Get("limit"))

	// Runs even for sort=recent, which ignores the value, so cursor validation
	// is independent of sort.
	cursor, ok := decodeNearPointCursor(q.Get("cursor"))
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), nearPointTimeout)
	defer cancel()

	var (
		apps       []PlanningApplication
		nextCursor string
		err        error
	)
	if sort == nearPointSortRecent {
		apps, err = h.store.RecentNearPoint(ctx, lat, lng, radius, limit)
	} else {
		apps, nextCursor, err = h.store.FindNearbyPage(ctx, lat, lng, radius, limit, cursor)
	}
	if err != nil {
		serverError(w, r, h.logger, "find applications near point", err)
		return
	}

	results := make([]NearbyResult, 0, len(apps))
	for _, a := range apps {
		result := NearbyResultOf(a)
		result.AuthoritySlug = resolveAuthoritySlugFor(r.Context(), h.resolver, h.logger, "near-point authority slug", a)
		results = append(results, result)
	}

	// Must precede writeJSON, which calls WriteHeader.
	if nextCursor != "" {
		w.Header().Set("X-Next-Cursor", encodeNearPointCursor(nextCursor))
	}
	writeJSON(w, r, h.logger, results)
}

// strconv.ParseFloat accepts "NaN" and "Inf", so the finiteness check is
// required even when parsing succeeds.
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

// parseNearPointRadius clamps rather than rejects: unparseable falls back to
// the default, out-of-range values (including zero or negative) are clamped.
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

// parseNearPointLimit clamps rather than rejects, like parseNearPointRadius.
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

// parseNearPointSort rejects unknown values: unlike radius and limit, there is
// no sensible value to clamp to.
func parseNearPointSort(raw string) (string, bool) {
	switch raw {
	case "", nearPointSortDistance:
		return nearPointSortDistance, true
	case nearPointSortRecent:
		return nearPointSortRecent, true
	default:
		return "", false
	}
}

// decodeNearPointCursor deliberately parallels watchzones' unexported cursor
// codec. A malformed value is rejected so a garbage cursor is a clean 400, not
// a silent reset to the first page.
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

func encodeNearPointCursor(token string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(token))
}
