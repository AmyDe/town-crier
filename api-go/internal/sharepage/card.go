package sharepage

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/designtokens"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// tileClient is the consumer-side seam over an OSM raster-tile source: the map
// compositor fetches the tiles overlapping the card window and draws them onto
// the canvas. *OSMTileClient (tileclient.go) satisfies it in production; tests
// pass a flat-colour fake. Fetch takes ctx first — it does network I/O.
type tileClient interface {
	Fetch(ctx context.Context, z, x, y int) (image.Image, error)
}

// Card geometry. The 1200x630 window is the Open Graph "summary_large_image"
// aspect; tiles are the OSM standard 256px; zoom 15 is a neighbourhood view that
// shows the street context of a site without leaking a precise pin-drop.
const (
	cardWidth  = 1200
	cardHeight = 630
	tileSize   = 256
	mapZoom    = 15

	osmAttribution = "© OpenStreetMap contributors"
)

var (
	pinFill    = designtokens.AmberDark  // amber pin body
	pinOutline = designtokens.White      // white pin border
	fallbackBg = designtokens.AmberLight // brand amber field
	textWhite  = designtokens.White
	// attribShade is a half-opaque black strip behind the OSM credit so it stays
	// legible over any tile. color.RGBA is alpha-premultiplied, so a black tint at
	// alpha 0xA0 darkens whatever it composites over via draw.Over.
	attribShade = color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xA0}
)

// The Go fonts are embedded, compile-time-constant assets; parsing them can only
// fail if those bytes are corrupt, which is a build-time programmer error, not a
// runtime condition — the same category as template.Must in templates.go. Parse
// once at package init rather than per request.
var (
	regularFont = mustParseFont(goregular.TTF)
	boldFont    = mustParseFont(gobold.TTF)
)

func mustParseFont(ttf []byte) *opentype.Font {
	f, err := opentype.Parse(ttf)
	if err != nil {
		panic("sharepage: parse embedded font: " + err.Error())
	}
	return f
}

// projectToGlobalPixel maps a WGS84 (lat, lng) to its global pixel coordinate at
// zoom z under the standard Web-Mercator slippy-map scheme: the world at zoom z
// is (2^z * tileSize) pixels square, and tile (x, y) owns pixels
// [x*tileSize, (x+1)*tileSize). See the OSM "Slippy map tilenames" spec.
func projectToGlobalPixel(lat, lng float64, z int) (px, py float64) {
	n := math.Exp2(float64(z))
	latRad := lat * math.Pi / 180
	x := (lng + 180) / 360 * n
	y := (1 - math.Log(math.Tan(latRad)+1/math.Cos(latRad))/math.Pi) / 2 * n
	return x * tileSize, y * tileSize
}

// generateCard renders the 1200x630 og:image PNG for an application. With both
// coordinates present it bakes an OSM map centred on the site with a pin; with
// either coordinate absent it returns the branded fallback card WITHOUT touching
// the tile client, so an uncoordinated application costs no OSM traffic.
func generateCard(ctx context.Context, tiles tileClient, app applications.PlanningApplication) ([]byte, error) {
	if app.Latitude == nil || app.Longitude == nil {
		return fallbackCard()
	}
	return mapCard(ctx, tiles, *app.Latitude, *app.Longitude)
}

// mapCard composites the OSM tiles overlapping a 1200x630 window centred on
// (lat, lng), then burns a pin at the centre and the OSM credit bottom-right.
// Tiles are fetched sequentially: the OSM tile-usage policy forbids parallel
// hammering, and cache-once keeps the per-app volume to one pass.
func mapCard(ctx context.Context, tiles tileClient, lat, lng float64) ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, cardWidth, cardHeight))

	centerPxX, centerPxY := projectToGlobalPixel(lat, lng, mapZoom)
	// Top-left of the window in global pixel space, and its integer pixel origin
	// used to place each tile.
	originX := centerPxX - cardWidth/2
	originY := centerPxY - cardHeight/2
	roundOriginX := int(math.Round(originX))
	roundOriginY := int(math.Round(originY))

	worldTiles := int(math.Exp2(mapZoom))
	minTileX := int(math.Floor(originX / tileSize))
	maxTileX := int(math.Floor((originX + cardWidth) / tileSize))
	minTileY := int(math.Floor(originY / tileSize))
	maxTileY := int(math.Floor((originY + cardHeight) / tileSize))

	for ty := minTileY; ty <= maxTileY; ty++ {
		if ty < 0 || ty >= worldTiles {
			continue // above the north pole / below the south pole: no tile exists
		}
		for tx := minTileX; tx <= maxTileX; tx++ {
			if tx < 0 || tx >= worldTiles {
				continue // off the antimeridian at this zoom: leave the gap blank
			}
			tile, err := tiles.Fetch(ctx, mapZoom, tx, ty)
			if err != nil {
				return nil, fmt.Errorf("fetch tile z%d/%d/%d: %w", mapZoom, tx, ty, err)
			}
			destX := tx*tileSize - roundOriginX
			destY := ty*tileSize - roundOriginY
			draw.Draw(canvas, image.Rect(destX, destY, destX+tileSize, destY+tileSize), tile, image.Point{}, draw.Src)
		}
	}

	drawPin(canvas, cardWidth/2, cardHeight/2)
	drawAttribution(canvas)

	return encodePNG(canvas)
}

// fallbackCard is the branded 1200x630 PNG served when an application has no
// coordinates: a solid brand-amber field with the "Town Crier" wordmark and a
// short tagline, both horizontally centred. It fetches no tiles.
func fallbackCard() ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, cardWidth, cardHeight))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(fallbackBg), image.Point{}, draw.Src)

	if err := drawCentredText(canvas, boldFont, "Town Crier", 72, cardHeight/2-16); err != nil {
		return nil, err
	}
	if err := drawCentredText(canvas, regularFont, "Planning alerts for your area", 30, cardHeight/2+52); err != nil {
		return nil, err
	}
	return encodePNG(canvas)
}

// drawPin renders a location pin whose tip points at (tipX, tipY): an amber
// teardrop with a white outline, ~37px tall, legible over any tile. It is the
// union of a head disc and a downward triangular tail — drawn white first, then
// amber inset by the outline width so a clean border remains.
func drawPin(img *image.RGBA, tipX, tipY int) {
	const (
		headRadius = 13
		tailLength = 24
		outline    = 3
	)
	headCenterY := tipY - tailLength
	fillPin(img, tipX, tipY+outline, headCenterY, headRadius+outline, pinOutline)
	fillPin(img, tipX, tipY, headCenterY, headRadius, pinFill)
}

// fillPin fills one pin shape in col: a head disc centred at (tipX, headCenterY)
// of the given radius, plus a triangular tail tapering from the disc's horizontal
// diameter down to (tipX, tipY).
func fillPin(img *image.RGBA, tipX, tipY, headCenterY, radius int, col color.RGBA) {
	fillDisc(img, tipX, headCenterY, radius, col)
	span := float64(tipY - headCenterY)
	for y := headCenterY; y <= tipY; y++ {
		halfW := int(math.Round(float64(radius) * float64(tipY-y) / span))
		for x := tipX - halfW; x <= tipX+halfW; x++ {
			setPixel(img, x, y, col)
		}
	}
}

// fillDisc fills the disc of radius r centred at (cx, cy) with col.
func fillDisc(img *image.RGBA, cx, cy, r int, col color.RGBA) {
	r2 := r * r
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= r2 {
				setPixel(img, x, y, col)
			}
		}
	}
}

// setPixel writes col at (x, y) if that point is inside the image.
func setPixel(img *image.RGBA, x, y int, col color.RGBA) {
	if image.Pt(x, y).In(img.Bounds()) {
		img.SetRGBA(x, y, col)
	}
}

// drawAttribution burns the OSM credit into the bottom-right corner over a
// semi-transparent dark strip sized from the measured string. Attribution is
// best-effort: a face-construction failure must not sink the whole card, so it
// returns silently rather than erroring.
func drawAttribution(img *image.RGBA) {
	const (
		fontPx  = 14
		padX    = 8
		padY    = 4
		marginR = 6
		marginB = 6
	)
	face, err := newFace(regularFont, fontPx)
	if err != nil {
		return
	}
	defer func() { _ = face.Close() }()

	width := font.MeasureString(face, osmAttribution).Ceil()
	metrics := face.Metrics()
	ascent := metrics.Ascent.Ceil()
	textH := ascent + metrics.Descent.Ceil()

	stripW := width + 2*padX
	stripH := textH + 2*padY
	stripX0 := cardWidth - marginR - stripW
	stripY0 := cardHeight - marginB - stripH

	draw.Draw(img,
		image.Rect(stripX0, stripY0, stripX0+stripW, stripY0+stripH),
		image.NewUniform(attribShade), image.Point{}, draw.Over)

	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(textWhite),
		Face: face,
		Dot:  fixed.P(stripX0+padX, stripY0+padY+ascent),
	}
	drawer.DrawString(osmAttribution)
}

// drawCentredText draws text in white, horizontally centred, with its baseline at
// baselineY, using font f at sizePx pixels.
func drawCentredText(img *image.RGBA, f *opentype.Font, text string, sizePx float64, baselineY int) error {
	face, err := newFace(f, sizePx)
	if err != nil {
		return fmt.Errorf("build font face: %w", err)
	}
	defer func() { _ = face.Close() }()

	width := font.MeasureString(face, text).Ceil()
	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(textWhite),
		Face: face,
		Dot:  fixed.P((cardWidth-width)/2, baselineY),
	}
	drawer.DrawString(text)
	return nil
}

// newFace builds an anti-aliased face for f at sizePx pixels. DPI is fixed at 72
// so one point equals one pixel and Size is read directly as the pixel height.
func newFace(f *opentype.Font, sizePx float64) (font.Face, error) {
	return opentype.NewFace(f, &opentype.FaceOptions{
		Size:    sizePx,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

// encodePNG encodes img to PNG bytes.
func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buf.Bytes(), nil
}
