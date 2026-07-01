package sharepage

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"time"
)

const (
	// osmTileBaseURL is the OSM standard raster-tile endpoint. The tile-usage
	// policy requires an identifying User-Agent and forbids bulk/parallel use;
	// cache-once keeps our volume to one sequential pass per shared application.
	osmTileBaseURL = "https://tile.openstreetmap.org"
	osmUserAgent   = "TownCrier/1.0 (+https://towncrierapp.uk)"

	tileFetchTimeout = 10 * time.Second
	maxTileBytes     = 1 << 20 // 1 MiB; an OSM 256px PNG tile is a few KiB
)

// errTileStatus is returned when a tile endpoint answers non-200 — a permanent
// failure for that tile (typically a missing tile at this zoom).
var errTileStatus = errors.New("unexpected tile status")

// OSMTileClient fetches OSM raster tiles over HTTPS with an identifying
// User-Agent, honouring the caller's context. It satisfies the tileClient seam
// consumed by the map compositor.
type OSMTileClient struct {
	http    *http.Client
	baseURL string
}

// NewOSMTileClient builds the production tile client against
// tile.openstreetmap.org with a bounded HTTP client. Construction opens no
// connections (they open lazily on first fetch), so wiring can build it
// unconditionally.
func NewOSMTileClient() *OSMTileClient {
	return newOSMTileClient(osmTileBaseURL, &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     90 * time.Second,
		},
	})
}

// newOSMTileClient is the seam the tests use to point the client at a local
// httptest server.
func newOSMTileClient(baseURL string, httpClient *http.Client) *OSMTileClient {
	return &OSMTileClient{http: httpClient, baseURL: baseURL}
}

// Fetch GETs the z/x/y tile and decodes it. It bounds the request time (a
// per-tile timeout on top of the client timeout) and the response body, and
// treats any non-200 as a permanent error for that tile.
func (c *OSMTileClient) Fetch(ctx context.Context, z, x, y int) (image.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, tileFetchTimeout)
	defer cancel()

	// gosec G107: the host is the constant osmTileBaseURL and the path is built
	// from integer tile coordinates only — no external free-text in the URL.
	url := fmt.Sprintf("%s/%d/%d/%d.png", c.baseURL, z, x, y)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //nolint:gosec // constant host + integer coordinates
	if err != nil {
		return nil, fmt.Errorf("build tile request %d/%d/%d: %w", z, x, y, err)
	}
	req.Header.Set("User-Agent", osmUserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get tile %d/%d/%d: %w", z, x, y, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get tile %d/%d/%d: %w: status %d", z, x, y, errTileStatus, resp.StatusCode)
	}

	img, err := png.Decode(io.LimitReader(resp.Body, maxTileBytes))
	if err != nil {
		return nil, fmt.Errorf("decode tile %d/%d/%d: %w", z, x, y, err)
	}
	return img, nil
}
