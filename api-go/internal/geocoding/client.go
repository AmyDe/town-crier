package geocoding

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// maxRespBytes bounds the postcodes.io response body read.
const maxRespBytes = 1 << 20

// Coordinates is the geocoded location, mirroring .NET Coordinates: { latitude,
// longitude }.
type Coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Client is the postcodes.io outbound geocoder. The base URL is config-supplied
// (default https://api.postcodes.io/), so no user input reaches the scheme.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient builds a geocoder over the given base URL and shared HTTP client.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: httpClient}
}

// postcodesIoResponse mirrors the postcodes.io envelope.
type postcodesIoResponse struct {
	Status int                `json:"status"`
	Result *postcodesIoResult `json:"result"`
}

type postcodesIoResult struct {
	Postcode      string  `json:"postcode"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	AdminDistrict *string `json:"admin_district"`
}

// Geocode resolves a normalised UK postcode to coordinates. The boolean reports
// whether the postcode was found: a non-2xx response or an envelope that is not
// status 200 with a result yields found=false (a 404 for the caller), mirroring
// .NET's null return. A transport failure returns an error (a 500 for the
// caller), mirroring .NET's propagated HttpRequestException.
func (c *Client) Geocode(ctx context.Context, postcode string) (Coordinates, bool, error) {
	endpoint := c.baseURL + "/postcodes/" + url.PathEscape(postcode)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Coordinates{}, false, fmt.Errorf("build geocode request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Coordinates{}, false, fmt.Errorf("geocode request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Coordinates{}, false, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return Coordinates{}, false, fmt.Errorf("read geocode response: %w", err)
	}
	var parsed postcodesIoResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Coordinates{}, false, fmt.Errorf("decode geocode response: %w", err)
	}
	if parsed.Status != 200 || parsed.Result == nil {
		return Coordinates{}, false, nil
	}
	return Coordinates{Latitude: parsed.Result.Latitude, Longitude: parsed.Result.Longitude}, true, nil
}
