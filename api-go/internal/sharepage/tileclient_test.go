package sharepage

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
)

// writeTilePNG writes a solid 256x256 PNG, standing in for an OSM tile endpoint.
func writeTilePNG(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, tileSize, tileSize))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xFF}), image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode tile: %v", err)
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(buf.Bytes())
}

// TestOSMTileClient_Fetch_DecodesTileAndSendsUserAgent drives the real client
// against a local server: it must GET /{z}/{x}/{y}.png, send the identifying
// User-Agent the OSM tile policy requires, and decode the PNG body.
func TestOSMTileClient_Fetch_DecodesTileAndSendsUserAgent(t *testing.T) {
	t.Parallel()
	var gotPath, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUA = r.Header.Get("User-Agent")
		writeTilePNG(t, w)
	}))
	defer srv.Close()

	c := newOSMTileClient(srv.URL, srv.Client())
	img, err := c.Fetch(t.Context(), 15, 16368, 10896)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if img.Bounds().Dx() != tileSize || img.Bounds().Dy() != tileSize {
		t.Errorf("tile bounds = %v, want %dx%d", img.Bounds(), tileSize, tileSize)
	}
	if gotPath != "/15/16368/10896.png" {
		t.Errorf("request path = %q, want /15/16368/10896.png", gotPath)
	}
	if gotUA != "TownCrier/1.0 (+https://towncrierapp.uk)" {
		t.Errorf("User-Agent = %q, want the identifying TownCrier UA", gotUA)
	}
}

// TestOSMTileClient_Fetch_NonOKStatus_Errors treats a non-200 tile response as a
// failure so a broken tile surfaces as a card render error, not a blank canvas.
func TestOSMTileClient_Fetch_NonOKStatus_Errors(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newOSMTileClient(srv.URL, srv.Client())
	if _, err := c.Fetch(t.Context(), 15, 1, 1); err == nil {
		t.Fatal("expected an error on a non-200 tile response")
	}
}

// TestOSMTileClient_SatisfiesTileClient pins that the exported production client
// satisfies the compositor's tileClient seam, so wiring can pass it straight in.
func TestOSMTileClient_SatisfiesTileClient(t *testing.T) {
	t.Parallel()
	var _ tileClient = NewOSMTileClient()
}
