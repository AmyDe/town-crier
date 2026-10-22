package sharepage

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// fakeImageStore is a hand-written double for the share-card blob cache. It
// records Get/Put calls so the cache-once invariant is checkable, and can be
// seeded to force a hit or to fail either operation.
type fakeImageStore struct {
	data     map[string][]byte
	getErr   error
	putErr   error
	getCalls int
	putCalls int
}

func newFakeImageStore() *fakeImageStore {
	return &fakeImageStore{data: map[string][]byte{}}
}

func (f *fakeImageStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	f.getCalls++
	if f.getErr != nil {
		return nil, false, f.getErr
	}
	b, ok := f.data[key]
	return b, ok, nil
}

func (f *fakeImageStore) Put(_ context.Context, key string, png []byte) error {
	f.putCalls++
	if f.putErr != nil {
		return f.putErr
	}
	f.data[key] = png
	return nil
}

// coordApp is fullApp with coordinates set, so the map path (not the fallback)
// runs.
func coordApp(t *testing.T) applications.PlanningApplication {
	t.Helper()
	app := fullApp(t)
	app.Latitude = ptr(51.5074)
	app.Longitude = ptr(-0.1278)
	return app
}

// serveOG drives a path through a pre-wired mux and returns the recorder.
func serveOG(t *testing.T, mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	mux.ServeHTTP(rec, req)
	return rec
}

// wireOG builds a mux with the og:image route wired to the given doubles.
func wireOG(t *testing.T, store appStore, resolver slugResolver, tiles tileClient, cache imageStore) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	ImageRoutes(mux, store, resolver, tiles, cache, slog.New(slog.DiscardHandler))
	return mux
}

func TestImageServe_ValidRequest_RendersMapPNG(t *testing.T) {
	t.Parallel()
	store := &fakeStore{app: coordApp(t), found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}
	tiles := &fakeTiles{fill: color.RGBA{R: 0x88, G: 0xCC, B: 0xFF, A: 0xFF}}
	cache := newFakeImageStore()

	rec := serveOG(t, wireOG(t, store, resolver, tiles, cache), "/og/croydon/23/03456/FUL.png")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want public, max-age=86400", got)
	}
	// The .png suffix is stripped before the point read: the store sees the bare
	// ref, keyed by the resolved (stringified) area id.
	if store.gotAuthorityCode != "165" || store.gotName != "23/03456/FUL" {
		t.Errorf("store queried with (%q,%q), want (165, 23/03456/FUL)", store.gotAuthorityCode, store.gotName)
	}
	img := decodePNG(t, rec.Body.Bytes())
	if img.Bounds().Dx() != cardWidth || img.Bounds().Dy() != cardHeight {
		t.Errorf("bounds = %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), cardWidth, cardHeight)
	}
	if tiles.calls < 1 {
		t.Errorf("tile fetches = %d, want >= 1 on a cache miss", tiles.calls)
	}
	if cache.putCalls != 1 {
		t.Errorf("cache Put called %d times, want exactly 1 on a miss", cache.putCalls)
	}
}

func TestImageServe_CacheOnce_SecondRequestSkipsTiles(t *testing.T) {
	t.Parallel()
	store := &fakeStore{app: coordApp(t), found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}
	tiles := &fakeTiles{fill: color.RGBA{R: 0x88, G: 0xCC, B: 0xFF, A: 0xFF}}
	cache := newFakeImageStore()
	mux := wireOG(t, store, resolver, tiles, cache)

	first := serveOG(t, mux, "/og/croydon/23/03456/FUL.png")
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", first.Code)
	}
	if tiles.calls < 1 {
		t.Fatalf("first request fetched %d tiles, want >= 1", tiles.calls)
	}
	if cache.putCalls != 1 {
		t.Fatalf("first request Put %d times, want 1", cache.putCalls)
	}
	firstBody := bytes.Clone(first.Body.Bytes())

	tiles.calls = 0
	getsBefore := cache.getCalls

	second := serveOG(t, mux, "/og/croydon/23/03456/FUL.png")
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200", second.Code)
	}
	// Cache-once: the second request must be served from the store without ever
	// touching the tile client again, and must not re-Put.
	if tiles.calls != 0 {
		t.Errorf("second request fetched %d tiles, want 0 (served from cache)", tiles.calls)
	}
	if cache.getCalls <= getsBefore {
		t.Errorf("second request did not read the cache (getCalls=%d, before=%d)", cache.getCalls, getsBefore)
	}
	if cache.putCalls != 1 {
		t.Errorf("cache Put called %d times total, want 1 (no re-Put on a hit)", cache.putCalls)
	}
	if !bytes.Equal(second.Body.Bytes(), firstBody) {
		t.Error("second body differs from the stored bytes; cache did not serve the baked card")
	}
}

// TestImageServe_NilCache_RendersByteIdenticalPNGAcrossTwoRequests is a
// characterisation guard for the v2 restyle (tc-fmrx3, #855): the masthead
// strip and pin-outline token swap must stay fully deterministic, and the
// og:image route's caching mechanism must still produce byte-stable output
// across two renders of the same application, exactly as it did before the
// restyle. A nil cache regenerates on every request (Slice 2, pre-Blob), so
// this drives the rendering path twice and asserts the bytes match, without
// touching ogimage.go's caching/routing logic at all.
func TestImageServe_NilCache_RendersByteIdenticalPNGAcrossTwoRequests(t *testing.T) {
	t.Parallel()
	store := &fakeStore{app: coordApp(t), found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}
	tiles := &fakeTiles{fill: color.RGBA{R: 0x88, G: 0xCC, B: 0xFF, A: 0xFF}}
	mux := wireOG(t, store, resolver, tiles, nil)

	first := serveOG(t, mux, "/og/croydon/23/03456/FUL.png")
	second := serveOG(t, mux, "/og/croydon/23/03456/FUL.png")

	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("statuses = %d, %d, want 200, 200", first.Code, second.Code)
	}
	if !bytes.Equal(first.Body.Bytes(), second.Body.Bytes()) {
		t.Error("regenerated PNG bytes differ across two renders of the same application; the restyle must stay deterministic")
	}
}

func TestImageServe_NoCoordinates_FallbackWithoutTiles(t *testing.T) {
	t.Parallel()
	store := &fakeStore{app: fullApp(t), found: true} // fullApp has no coordinates
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}
	tiles := &fakeTiles{fill: color.RGBA{R: 0x88, G: 0xCC, B: 0xFF, A: 0xFF}}

	rec := serveOG(t, wireOG(t, store, resolver, tiles, newFakeImageStore()), "/og/croydon/23/03456/FUL.png")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", got)
	}
	if tiles.calls != 0 {
		t.Errorf("tile fetches = %d, want 0 for a coordinate-less app (branded fallback)", tiles.calls)
	}
	img := decodePNG(t, rec.Body.Bytes())
	if img.Bounds().Dx() != cardWidth || img.Bounds().Dy() != cardHeight {
		t.Errorf("bounds = %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), cardWidth, cardHeight)
	}
}

func TestImageServe_NilCache_RegeneratesEveryRequest(t *testing.T) {
	t.Parallel()
	store := &fakeStore{app: coordApp(t), found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}
	tiles := &fakeTiles{fill: color.RGBA{R: 0x88, G: 0xCC, B: 0xFF, A: 0xFF}}
	// nil cache: the Slice-3 Blob store is not wired yet; degrade to regenerate.
	mux := wireOG(t, store, resolver, tiles, nil)

	first := serveOG(t, mux, "/og/croydon/23/03456/FUL.png")
	afterFirst := tiles.calls
	second := serveOG(t, mux, "/og/croydon/23/03456/FUL.png")

	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("statuses = %d, %d, want 200, 200", first.Code, second.Code)
	}
	if afterFirst < 1 {
		t.Errorf("first request fetched %d tiles, want >= 1", afterFirst)
	}
	if tiles.calls <= afterFirst {
		t.Errorf("second request did not regenerate: tile fetches stayed at %d (nil cache must not memoise)", tiles.calls)
	}
}

func TestImageServe_CacheReadError_Regenerates(t *testing.T) {
	t.Parallel()
	store := &fakeStore{app: coordApp(t), found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}
	tiles := &fakeTiles{fill: color.RGBA{R: 0x88, G: 0xCC, B: 0xFF, A: 0xFF}}
	cache := newFakeImageStore()
	cache.getErr = errors.New("blob unavailable")

	rec := serveOG(t, wireOG(t, store, resolver, tiles, cache), "/og/croydon/23/03456/FUL.png")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (a cache read error must not fail the request)", rec.Code)
	}
	if tiles.calls < 1 {
		t.Errorf("tile fetches = %d, want >= 1 (regenerate after a cache read error)", tiles.calls)
	}
}

func TestImageServe_UnknownSlug_NotFoundNoStore(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}
	tiles := &fakeTiles{}

	rec := serveOG(t, wireOG(t, store, resolver, tiles, newFakeImageStore()), "/og/nowhere/23/0001/FUL.png")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if store.calls != 0 {
		t.Errorf("store queried %d times for an unknown slug, want 0", store.calls)
	}
	if tiles.calls != 0 {
		t.Errorf("tile fetches = %d, want 0 for an unknown slug", tiles.calls)
	}
	if strings.Contains(rec.Header().Get("Cache-Control"), "max-age") {
		t.Errorf("404 Cache-Control = %q must not positively cache", rec.Header().Get("Cache-Control"))
	}
}

func TestImageServe_UnknownRef_NotFound(t *testing.T) {
	t.Parallel()
	store := &fakeStore{found: false}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}
	tiles := &fakeTiles{}

	rec := serveOG(t, wireOG(t, store, resolver, tiles, newFakeImageStore()), "/og/croydon/99/9999/XXX.png")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if tiles.calls != 0 {
		t.Errorf("tile fetches = %d, want 0 for an unknown ref", tiles.calls)
	}
}

func TestImageServe_NonPNGSuffix_NotFound(t *testing.T) {
	t.Parallel()
	store := &fakeStore{app: coordApp(t), found: true}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}
	tiles := &fakeTiles{}

	// No .png suffix: the mux matches the route, but the handler rejects it before
	// any resolve/read/render.
	rec := serveOG(t, wireOG(t, store, resolver, tiles, newFakeImageStore()), "/og/croydon/23/03456/FUL")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for a non-.png request", rec.Code)
	}
	if store.calls != 0 {
		t.Errorf("store queried %d times for a non-.png request, want 0", store.calls)
	}
	if tiles.calls != 0 {
		t.Errorf("tile fetches = %d, want 0 for a non-.png request", tiles.calls)
	}
}

func TestImageServe_StoreError_Returns500(t *testing.T) {
	t.Parallel()
	store := &fakeStore{err: errors.New("db down")}
	resolver := &fakeResolver{slugs: map[string]int{"croydon": 165}}
	tiles := &fakeTiles{}

	rec := serveOG(t, wireOG(t, store, resolver, tiles, newFakeImageStore()), "/og/croydon/23/0001/FUL.png")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if tiles.calls != 0 {
		t.Errorf("tile fetches = %d, want 0 when the store errors", tiles.calls)
	}
}

const ogPath = "/og/croydon/23/03456/FUL.png"

// gatedTiles counts fetches and the maximum number in flight. The first Fetch
// signals entered and blocks until release is closed.
type gatedTiles struct {
	calls    atomic.Int32
	inFlight atomic.Int32
	maxSeen  atomic.Int32
	entered  chan struct{}
	release  chan struct{}
	once     sync.Once
}

func newGatedTiles() *gatedTiles {
	return &gatedTiles{entered: make(chan struct{}), release: make(chan struct{})}
}

func (g *gatedTiles) Fetch(ctx context.Context, _, _, _ int) (image.Image, error) {
	g.calls.Add(1)
	n := g.inFlight.Add(1)
	defer g.inFlight.Add(-1)
	for {
		m := g.maxSeen.Load()
		if n <= m || g.maxSeen.CompareAndSwap(m, n) {
			break
		}
	}
	g.once.Do(func() {
		close(g.entered)
		select {
		case <-g.release:
		case <-ctx.Done():
		}
	})
	img := image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{R: 0x88, G: 0xCC, B: 0xFF, A: 0xFF}), image.Point{}, draw.Src)
	return img, nil
}

// scriptedCache returns the scripted Get results in order (the last repeats),
// counts Puts, and signals each Get on gets.
type scriptedCache struct {
	results []bool
	png     []byte
	gets    atomic.Int32
	puts    atomic.Int32
	getSig  chan struct{}
}

func (c *scriptedCache) Get(_ context.Context, _ string) ([]byte, bool, error) {
	i := int(c.gets.Add(1)) - 1
	if c.getSig != nil {
		c.getSig <- struct{}{}
	}
	if i >= len(c.results) {
		i = len(c.results) - 1
	}
	if c.results[i] {
		return c.png, true, nil
	}
	return nil, false, nil
}

func (c *scriptedCache) Put(_ context.Context, _ string, _ []byte) error {
	c.puts.Add(1)
	return nil
}

// staticStore is a concurrency-safe appStore returning one application.
type staticStore struct {
	app applications.PlanningApplication
}

func (s staticStore) GetByAuthorityAndName(context.Context, string, string) (applications.PlanningApplication, bool, error) {
	return s.app, true, nil
}

func newLimitedHandler(t *testing.T, tiles tileClient, cache imageStore, wait time.Duration) (*imageHandler, *http.ServeMux) {
	t.Helper()
	h := newImageHandler(
		staticStore{app: coordApp(t)},
		&fakeResolver{slugs: map[string]int{"croydon": 165}},
		tiles, cache, slog.New(slog.DiscardHandler), wait,
	)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /og/{authoritySlug}/{ref...}", h.serve)
	return h, mux
}

// serveSpanned serves path under a recording request span and returns the
// recorder plus the sharecard.cache attribute ("" when unset).
func serveSpanned(t *testing.T, mux *http.ServeMux) (*httptest.ResponseRecorder, string) {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.WithoutCancel(t.Context())) })
	spanCtx, span := tp.Tracer("test").Start(t.Context(), "request")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(spanCtx, http.MethodGet, ogPath, nil))
	span.End()

	ended := sr.Ended()
	if len(ended) != 1 {
		t.Fatalf("recorded spans = %d, want 1", len(ended))
	}
	for _, kv := range ended[0].Attributes() {
		if string(kv.Key) == "sharecard.cache" {
			return rec, kv.Value.AsString()
		}
	}
	return rec, ""
}

func TestImageServe_SlotHeld_Returns429WithoutFetching(t *testing.T) {
	t.Parallel()
	tiles := newGatedTiles()
	cache := &scriptedCache{results: []bool{false}}
	h, mux := newLimitedHandler(t, tiles, cache, 20*time.Millisecond)
	h.renderSlot <- struct{}{}

	rec, attr := serveSpanned(t, mux)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "120" {
		t.Errorf("Retry-After = %q, want 120", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body length = %d, want 0", rec.Body.Len())
	}
	if n := tiles.calls.Load(); n != 0 {
		t.Errorf("tile fetches = %d, want 0", n)
	}
	if attr != "busy" {
		t.Errorf("sharecard.cache = %q, want busy", attr)
	}
}

func TestImageServe_SlotHeldContextCancelled_ReturnsWithoutWaiting(t *testing.T) {
	t.Parallel()
	tiles := newGatedTiles()
	h, mux := newLimitedHandler(t, tiles, &scriptedCache{results: []bool{false}}, time.Hour)
	h.renderSlot <- struct{}{}
	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequestWithContext(ctx, http.MethodGet, ogPath, nil))
		done <- rec
	}()
	cancel()

	select {
	case rec := <-done:
		if rec.Code != http.StatusTooManyRequests {
			t.Errorf("status = %d, want 429", rec.Code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("handler did not return after context cancel")
	}
	if n := tiles.calls.Load(); n != 0 {
		t.Errorf("tile fetches = %d, want 0", n)
	}
}

func TestImageServe_SlotHeldCacheHit_ServesImmediately(t *testing.T) {
	t.Parallel()
	want := []byte("cached-png")
	cache := &scriptedCache{results: []bool{true}, png: want}
	tiles := newGatedTiles()
	h, mux := newLimitedHandler(t, tiles, cache, time.Hour)
	h.renderSlot <- struct{}{}

	rec, attr := serveSpanned(t, mux)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), want) {
		t.Error("body is not the cached PNG")
	}
	if attr != "hit" {
		t.Errorf("sharecard.cache = %q, want hit", attr)
	}
}

func TestImageServe_MissThenHitOnReread_ServesCachedWithoutRendering(t *testing.T) {
	t.Parallel()
	want := []byte("cached-png")
	cache := &scriptedCache{results: []bool{false, true}, png: want}
	tiles := newGatedTiles()
	_, mux := newLimitedHandler(t, tiles, cache, time.Second)

	rec, attr := serveSpanned(t, mux)

	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), want) {
		t.Fatalf("status = %d, body = %q; want 200 with cached bytes", rec.Code, rec.Body.String())
	}
	if n := tiles.calls.Load(); n != 0 {
		t.Errorf("tile fetches = %d, want 0", n)
	}
	if n := cache.puts.Load(); n != 0 {
		t.Errorf("cache writes = %d, want 0", n)
	}
	if attr != "hit" {
		t.Errorf("sharecard.cache = %q, want hit", attr)
	}
}

func TestImageServe_FreeSlotMiss_RendersOnceAndReleasesSlot(t *testing.T) {
	t.Parallel()
	tiles := newGatedTiles()
	close(tiles.release)
	cache := &scriptedCache{results: []bool{false}}
	h, mux := newLimitedHandler(t, tiles, cache, time.Second)

	rec, attr := serveSpanned(t, mux)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if tiles.calls.Load() < 1 {
		t.Error("expected tile fetches for a render")
	}
	if n := cache.puts.Load(); n != 1 {
		t.Errorf("cache writes = %d, want 1", n)
	}
	if attr != "miss" {
		t.Errorf("sharecard.cache = %q, want miss", attr)
	}
	select {
	case h.renderSlot <- struct{}{}:
	default:
		t.Fatal("render slot still held after the request")
	}
}

func TestImageServe_ConcurrentMisses_NeverRenderInParallel(t *testing.T) {
	t.Parallel()
	tiles := newGatedTiles()
	cache := &scriptedCache{results: []bool{false}, getSig: make(chan struct{}, 8)}
	_, mux := newLimitedHandler(t, tiles, cache, 30*time.Second)

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mux.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, ogPath, nil))
		}()
	}

	<-tiles.entered
	<-cache.getSig // first request's first read
	<-cache.getSig // first request's re-read after acquire
	<-cache.getSig // second request's first read, now queued for the slot
	close(tiles.release)
	wg.Wait()

	if got := tiles.maxSeen.Load(); got != 1 {
		t.Errorf("max concurrent Fetch = %d, want 1", got)
	}
}
