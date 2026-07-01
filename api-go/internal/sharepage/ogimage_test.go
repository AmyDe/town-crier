package sharepage

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
