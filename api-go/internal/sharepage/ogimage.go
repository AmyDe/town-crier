package sharepage

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	cacheAttr  = "sharecard.cache"
	busyRetry  = "120"
	renderWait = 5 * time.Second
)

// imageStore caches baked share-card PNGs by area-scoped key; the handler
// writes a card once on a miss.
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

	renderSlot chan struct{}
	renderWait time.Duration
}

// ImageRoutes registers the anonymous og:image route. The {ref...} wildcard MUST
// be the final path segment, so the ".png" suffix cannot be baked into the mux
// pattern — registering "{ref...}.png" panics — and the handler enforces and
// strips it instead. The public URL shape stays /og/{slug}/{ref}.png. Keep the
// pattern in lockstep with the anonymousPatterns entry in cmd/api/wiring.go.
//
// cache may be nil to disable caching; pass an untyped nil interface, not a
// typed-nil pointer.
func ImageRoutes(mux *http.ServeMux, store appStore, resolver slugResolver, tiles tileClient, cache imageStore, logger *slog.Logger) {
	h := newImageHandler(store, resolver, tiles, cache, logger, renderWait)
	mux.HandleFunc("GET /og/{authoritySlug}/{ref...}", h.serve)
}

func newImageHandler(store appStore, resolver slugResolver, tiles tileClient, cache imageStore, logger *slog.Logger, wait time.Duration) *imageHandler {
	return &imageHandler{
		store: store, resolver: resolver, tiles: tiles, cache: cache, logger: logger,
		renderSlot: make(chan struct{}, 1),
		renderWait: wait,
	}
}

// serve resolves the slug, point-reads the application, then serves the baked
// card — from the cache when present, otherwise generating it (map or fallback)
// and caching it. An unknown slug/ref or a non-.png request is a no-store 404
// (never a 500); a store error is a bodyless 500 (the envelope is backfilled by
// middleware). Cache hits never wait; a miss renders under a one-per-replica
// slot (OSM tile policy) and answers 429 + Retry-After when the slot is not free
// within renderWait.
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

	span := trace.SpanFromContext(r.Context())

	if cached, ok := h.readCache(r.Context(), key); ok {
		span.SetAttributes(attribute.String(cacheAttr, "hit"))
		h.writePNG(r.Context(), w, cached)
		return
	}

	if !h.acquire(r) {
		span.SetAttributes(attribute.String(cacheAttr, "busy"))
		h.logger.InfoContext(r.Context(), "share card render busy", "key", key)
		w.Header().Set("Retry-After", busyRetry)
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}
	defer func() { <-h.renderSlot }()

	// Another request may have rendered this key while we queued for the slot.
	if cached, ok := h.readCache(r.Context(), key); ok {
		span.SetAttributes(attribute.String(cacheAttr, "hit"))
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
	span.SetAttributes(attribute.String(cacheAttr, "miss"))
	h.writePNG(r.Context(), w, png)
}

// acquire takes the render slot, giving up after renderWait or when the request
// context ends.
func (h *imageHandler) acquire(r *http.Request) bool {
	timer := time.NewTimer(h.renderWait)
	defer timer.Stop()
	select {
	case h.renderSlot <- struct{}{}:
		return true
	case <-timer.C:
		return false
	case <-r.Context().Done():
		return false
	}
}

// readCache returns the cached card; a nil cache or a read error means regenerate.
func (h *imageHandler) readCache(ctx context.Context, key string) ([]byte, bool) {
	if h.cache == nil {
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
