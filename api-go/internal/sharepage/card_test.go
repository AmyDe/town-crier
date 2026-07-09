package sharepage

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/designtokens"
)

// fakeTiles is a hand-written double for the tile client. Every tile is a single
// flat colour so a test can assert that later drawing (the pin, the attribution
// strip) actually changed pixels away from the raw map. It counts Fetch calls so
// the cache-once and no-coordinates-no-fetch invariants are checkable.
type fakeTiles struct {
	fill  color.RGBA
	err   error
	calls int
}

func (f *fakeTiles) Fetch(_ context.Context, _, _, _ int) (image.Image, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	img := image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
	draw.Draw(img, img.Bounds(), image.NewUniform(f.fill), image.Point{}, draw.Src)
	return img, nil
}

// hasPixelUnlike reports whether any pixel in rect differs (in RGB) from c. Used
// to assert a drawing step ran without pinning exact glyph/pin pixels: the tiles
// are a flat fill, so any pin or attribution pixel differs from it.
func hasPixelUnlike(img image.Image, rect image.Rectangle, c color.RGBA) bool {
	wr, wg, wb, _ := c.RGBA()
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r != wr || g != wg || b != wb {
				return true
			}
		}
	}
	return false
}

// sameRGB reports whether the pixel at (x, y) matches c in RGB (ignoring alpha).
func sameRGB(t *testing.T, img image.Image, x, y int, c color.RGBA) bool {
	t.Helper()
	r, g, b, _ := img.At(x, y).RGBA()
	wr, wg, wb, _ := c.RGBA()
	return r == wr && g == wg && b == wb
}

// TestProjectToGlobalPixel_KnownVectors pins the Web-Mercator slippy-map
// projection against hand-derivable reference points. Each vector is analytic
// (equator/meridian cases where the log term collapses to zero), so it catches a
// regression in the formula independently of the implementation — a sign flip or
// a missing 2^z factor fails here rather than silently mis-centring every map.
func TestProjectToGlobalPixel_KnownVectors(t *testing.T) {
	t.Parallel()
	const eps = 1e-6
	tests := []struct {
		name           string
		lat, lng       float64
		z              int
		wantPx, wantPy float64
	}{
		// z=0: the whole world is one 256px tile; (0,0) sits dead centre.
		{"origin z0", 0, 0, 0, 128, 128},
		// z=1: the world is 2x2 tiles (512px); (0,0) is the shared corner.
		{"origin z1", 0, 0, 1, 256, 256},
		// Left edge of the world at the equator, z=1.
		{"antimeridian west z1", 0, -180, 1, 0, 256},
		// Right edge of the world at the equator, z=1.
		{"antimeridian east z1", 0, 180, 1, 512, 256},
		// z=2: 4x4 tiles (1024px); (0,0) is the centre.
		{"origin z2", 0, 0, 2, 512, 512},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			px, py := projectToGlobalPixel(tc.lat, tc.lng, tc.z)
			if math.Abs(px-tc.wantPx) > eps || math.Abs(py-tc.wantPy) > eps {
				t.Errorf("projectToGlobalPixel(%v,%v,%d) = (%v,%v), want (%v,%v)",
					tc.lat, tc.lng, tc.z, px, py, tc.wantPx, tc.wantPy)
			}
		})
	}
}

// TestProjectToGlobalPixel_NorthernLatitudeRaisesAboveEquator pins the sign of
// the mercator y term: a positive (northern) latitude must project ABOVE the
// equator, i.e. to a smaller y pixel than the equator's. Guards against dropping
// the leading (1 - ...) that flips the vertical axis.
func TestProjectToGlobalPixel_NorthernLatitudeRaisesAboveEquator(t *testing.T) {
	t.Parallel()
	_, equatorY := projectToGlobalPixel(0, -0.12, 15)
	_, londonY := projectToGlobalPixel(51.5074, -0.12, 15)
	if !(londonY < equatorY) {
		t.Errorf("northern latitude y=%v not above equator y=%v", londonY, equatorY)
	}
}

// TestMapCard_RendersMapWithPinAndAttribution drives the map path: tiles are
// fetched and composited to a 1200x630 PNG, then a pin is burned at the centre
// and the OSM credit bottom-right. The tiles are a flat sky colour, so the pin
// and attribution regions must contain pixels unlike that fill.
func TestMapCard_RendersMapWithPinAndAttribution(t *testing.T) {
	t.Parallel()
	fill := color.RGBA{0x88, 0xCC, 0xFF, 0xFF}
	tiles := &fakeTiles{fill: fill}

	data, err := mapCard(t.Context(), tiles, 51.5074, -0.1278)
	if err != nil {
		t.Fatalf("mapCard: %v", err)
	}
	img := decodePNG(t, data)

	if img.Bounds().Dx() != cardWidth || img.Bounds().Dy() != cardHeight {
		t.Fatalf("bounds = %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), cardWidth, cardHeight)
	}
	if tiles.calls < 1 {
		t.Errorf("tile fetches = %d, want >= 1", tiles.calls)
	}
	// Pin region around the window centre (600,315).
	if !hasPixelUnlike(img, image.Rect(584, 274, 617, 317), fill) {
		t.Error("no pin drawn: centre region is still the flat tile fill")
	}
	// Attribution region bottom-right.
	if !hasPixelUnlike(img, image.Rect(950, 595, 1195, 626), fill) {
		t.Error("no attribution drawn: bottom-right region is still the flat tile fill")
	}
}

// TestMapCard_RendersMastheadStripWithInkRule pins the Public Notice masthead
// band (tc-fmrx3, #855): a paper-coloured strip burned across the top of the
// map card, with a 2px ink rule beneath it, so the card carries the same
// wordmark-over-double-rule language as the web masthead (#853/#854) even
// though it composites onto a raster PNG rather than CSS. The band overlays
// the already-composited tiles rather than reserving a gap in the map
// projection, so this only changes THIS function's pixels — the map centring
// math and OSM tile budget are untouched.
func TestMapCard_RendersMastheadStripWithInkRule(t *testing.T) {
	t.Parallel()
	fill := color.RGBA{0x88, 0xCC, 0xFF, 0xFF}
	tiles := &fakeTiles{fill: fill}

	data, err := mapCard(t.Context(), tiles, 51.5074, -0.1278)
	if err != nil {
		t.Fatalf("mapCard: %v", err)
	}
	img := decodePNG(t, data)

	if !hasPixelUnlike(img, image.Rect(0, 0, cardWidth, mastheadHeight), fill) {
		t.Error("masthead band region still shows the raw tile fill")
	}
	// Away from the centred wordmark, the band is flat paper.
	if !sameRGB(t, img, 1100, 10, designtokens.SurfaceLight) {
		t.Error("masthead band is not the paper SurfaceLight colour")
	}
	// The ink rule beneath the band spans the full card width.
	if !sameRGB(t, img, 50, mastheadHeight, designtokens.TextPrimaryLight) {
		t.Error("masthead ink rule missing at its expected row (left)")
	}
	if !sameRGB(t, img, 1150, mastheadHeight, designtokens.TextPrimaryLight) {
		t.Error("masthead ink rule missing at its expected row (right)")
	}
}

// TestMapCard_PinOutlineIsPaperNotWhite pins the pin-outline token swap
// (tc-fmrx3, #855): the outline goes from pure white to the paper
// designtokens.SurfaceLight, consuming the token package rather than a
// hand-rolled white. (600,277) sits 14px from the pin head's centre (291),
// inside the outline ring (13px fill radius < 14 <= 16px outline radius).
func TestMapCard_PinOutlineIsPaperNotWhite(t *testing.T) {
	t.Parallel()
	tiles := &fakeTiles{fill: color.RGBA{0x88, 0xCC, 0xFF, 0xFF}}

	data, err := mapCard(t.Context(), tiles, 51.5074, -0.1278)
	if err != nil {
		t.Fatalf("mapCard: %v", err)
	}
	img := decodePNG(t, data)

	if !sameRGB(t, img, 600, 277, designtokens.SurfaceLight) {
		t.Error("pin outline is not the paper SurfaceLight colour")
	}
}

// TestMapCard_TileFetchError propagates a tile-client failure as an error rather
// than serving a half-painted card.
func TestMapCard_TileFetchError(t *testing.T) {
	t.Parallel()
	tiles := &fakeTiles{err: context.DeadlineExceeded}
	if _, err := mapCard(t.Context(), tiles, 51.5, -0.1); err == nil {
		t.Fatal("expected an error when a tile fetch fails")
	}
}

// TestFallbackCard_BrandedNoTiles pins the no-coordinates card: a solid
// brand-amber 1200x630 field with the wordmark burned in. It takes no tile
// client at all, so it cannot fetch.
func TestFallbackCard_BrandedNoTiles(t *testing.T) {
	t.Parallel()
	data, err := fallbackCard()
	if err != nil {
		t.Fatalf("fallbackCard: %v", err)
	}
	img := decodePNG(t, data)
	if img.Bounds().Dx() != cardWidth || img.Bounds().Dy() != cardHeight {
		t.Fatalf("bounds = %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), cardWidth, cardHeight)
	}
	// A corner is the flat brand-amber background.
	if !sameRGB(t, img, 4, 4, fallbackBg) {
		t.Error("corner pixel is not the brand-amber background")
	}
	// The centred wordmark drew white text over the amber field.
	if !hasPixelUnlike(img, image.Rect(400, 260, 800, 360), fallbackBg) {
		t.Error("no wordmark drawn: centre band is still the flat background")
	}
}

// TestFallbackCard_PaperFieldWithInkText pins the v2 fallback restyle
// (tc-fmrx3, #855): the flat brand-amber field becomes the paper
// designtokens.BackgroundLight, with ink (not white) wordmark text — the
// no-map card now reads as a filed notice rather than a brand-amber poster.
func TestFallbackCard_PaperFieldWithInkText(t *testing.T) {
	t.Parallel()
	data, err := fallbackCard()
	if err != nil {
		t.Fatalf("fallbackCard: %v", err)
	}
	img := decodePNG(t, data)

	if !sameRGB(t, img, 4, 4, designtokens.BackgroundLight) {
		t.Error("fallback corner is not the paper BackgroundLight field")
	}
	if sameRGB(t, img, 4, 4, designtokens.AmberLight) {
		t.Error("fallback corner is still the old flat brand-amber field")
	}
}

// TestGenerateCard_NoCoordinates_FallbackWithoutFetch pins the dispatch: an app
// with a nil coordinate takes the fallback path and never touches the tile
// client, so an uncoordinated application costs zero OSM traffic.
func TestGenerateCard_NoCoordinates_FallbackWithoutFetch(t *testing.T) {
	t.Parallel()
	tiles := &fakeTiles{fill: color.RGBA{0x88, 0xCC, 0xFF, 0xFF}}
	app := applications.PlanningApplication{Name: "24/0001/OUT", AreaID: 165} // no lat/lng

	data, err := generateCard(t.Context(), tiles, app)
	if err != nil {
		t.Fatalf("generateCard: %v", err)
	}
	if tiles.calls != 0 {
		t.Errorf("tile fetches = %d, want 0 for a coordinate-less app", tiles.calls)
	}
	img := decodePNG(t, data)
	if img.Bounds().Dx() != cardWidth || img.Bounds().Dy() != cardHeight {
		t.Errorf("bounds = %dx%d, want %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), cardWidth, cardHeight)
	}
}

// decodePNG is a shared test helper: it decodes b as a PNG and fails the test if
// it is not a valid PNG, returning the image for dimension/pixel assertions.
func decodePNG(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("bytes are not a valid PNG: %v", err)
	}
	return img
}
