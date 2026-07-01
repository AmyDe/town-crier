package sharepage

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

// imageStore is the consumer-side cache seam over the baked share-card PNGs: the
// image handler reads a card by its area-scoped key and writes it once on a miss.
// The production implementation is the Azure `share-cards` Blob container (Slice
// 3 infra, #738); until it is wired the handler runs with a nil store and simply
// regenerates on every request. Get takes ctx first — it does network I/O.
type imageStore interface {
	Get(ctx context.Context, key string) (png []byte, found bool, err error)
	Put(ctx context.Context, key string, png []byte) error
}

type imageHandler struct {
	store    appStore
	resolver slugResolver
	tiles    tileClient
	cache    imageStore // may be nil: degrade to regenerate-on-every-request
	logger   *slog.Logger
}

// ImageRoutes registers the anonymous og:image route. The {ref...} wildcard MUST
// be the final path segment, so the ".png" suffix cannot be baked into the mux
// pattern — registering "{ref...}.png" panics — and the handler enforces and
// strips it instead. The public URL shape stays /og/{slug}/{ref}.png. Keep the
// pattern in lockstep with the anonymousPatterns entry in cmd/api/wiring.go.
//
// cache may be nil: that is the deliberate degrade-gracefully seam for Slice 2,
// before the Blob container + managed-identity RBAC exist. Pass a genuine nil
// interface (not a typed-nil pointer) so the handler's nil-check disables caching
// cleanly.
func ImageRoutes(mux *http.ServeMux, store appStore, resolver slugResolver, tiles tileClient, cache imageStore, logger *slog.Logger) {
	h := &imageHandler{store: store, resolver: resolver, tiles: tiles, cache: cache, logger: logger}
	mux.HandleFunc("GET /og/{authoritySlug}/{ref...}", h.serve)
}

// serve resolves the slug, point-reads the application, then serves the baked
// card — from the cache when present, otherwise generating it (map or fallback)
// and caching it. An unknown slug/ref or a non-.png request is a no-store 404
// (never a 500); a store error is a bodyless 500 (the envelope is backfilled by
// middleware).
func (h *imageHandler) serve(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("authoritySlug")
	ref := r.PathValue("ref")

	// The mux cannot match the ".png" suffix (the wildcard must be final), so
	// require and strip it here. Anything else is an unknown resource, not a card.
	if !strings.HasSuffix(ref, ".png") {
		h.notFound(w)
		return
	}
	ref = strings.TrimSuffix(ref, ".png")

	areaID, ok := h.resolver.SlugToAreaID(slug)
	if !ok {
		h.notFound(w)
		return
	}

	app, found, err := h.store.GetByAuthorityAndName(r.Context(), strconv.Itoa(areaID), ref)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "share card read failed", "op", "read application by slug", "slug", slug, "ref", ref, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !found {
		h.notFound(w)
		return
	}

	// The cache key is area-id scoped and slug-agnostic: two slugs resolving to the
	// same area share one baked card, and a slug rename never orphans it.
	key := strconv.Itoa(areaID) + "/" + ref + ".png"

	if cached, ok := h.readCache(r.Context(), key); ok {
		h.writePNG(r.Context(), w, cached)
		return
	}

	png, err := generateCard(r.Context(), h.tiles, app)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "share card render failed", "key", key, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	h.writeCache(r.Context(), key, png)
	h.writePNG(r.Context(), w, png)
}

// readCache returns the baked card for key when a cache is wired and holds it. A
// nil cache (Slice 2, pre-Blob) logs the miss once and regenerates; a cache read
// error is non-fatal — the caller regenerates.
func (h *imageHandler) readCache(ctx context.Context, key string) ([]byte, bool) {
	if h.cache == nil {
		// The Blob container + MI RBAC land in Slice 3. Until then every request
		// regenerates; log it so the missing store stays visible without being
		// noisy (one line per served card).
		h.logger.InfoContext(ctx, "share card store unwired; regenerating on demand", "key", key)
		return nil, false
	}
	cached, found, err := h.cache.Get(ctx, key)
	if err != nil {
		h.logger.WarnContext(ctx, "share card cache read failed; regenerating", "key", key, "error", err)
		return nil, false
	}
	return cached, found
}

// writeCache stores a freshly generated card when a cache is wired. A write
// failure still serves this request; the next request retries.
func (h *imageHandler) writeCache(ctx context.Context, key string, png []byte) {
	if h.cache == nil {
		return
	}
	if err := h.cache.Put(ctx, key, png); err != nil {
		h.logger.WarnContext(ctx, "share card cache write failed", "key", key, "error", err)
	}
}

// writePNG serves the card body with a day-long public cache lifetime for the CF
// edge; the card is immutable for the application snapshot it depicts.
func (h *imageHandler) writePNG(ctx context.Context, w http.ResponseWriter, png []byte) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	// gosec G705: these are PNG bytes served with Content-Type image/png (a baked
	// map or the branded fallback), never HTML — not an XSS sink.
	if _, err := w.Write(png); err != nil { //nolint:gosec // PNG body served as image/png, not HTML
		h.logger.ErrorContext(ctx, "share card write failed", "error", err)
	}
}

// notFound emits a bodyless 404 that is never positively cached, so a
// just-ingested application is not negatively cached at the edge.
func (h *imageHandler) notFound(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
}
