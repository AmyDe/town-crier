package geocoding

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// authorityMappingJSON is the postcodes.io admin-district -> PlanIt authority-id
// table, embedded byte-for-byte from the .NET
// TownCrier.Infrastructure.Geocoding.authority-mapping.json resource so the Go
// resolver maps coordinates to the exact same authority ids.
//
//go:embed authority-mapping.json
var authorityMappingJSON []byte

// authorityMapping is the parsed admin-district -> authority-id table. Parsed
// once at package init; a malformed embedded resource is a programming error, so
// it panics at startup (the same approach as the authorities/legal packages).
var authorityMapping = loadAuthorityMapping()

func loadAuthorityMapping() map[string]int {
	var m map[string]int
	if err := json.Unmarshal(authorityMappingJSON, &m); err != nil {
		panic(fmt.Sprintf("geocoding: parse authority-mapping.json: %v", err))
	}
	return m
}

// ErrAuthorityUnresolved is returned when coordinates cannot be mapped to a
// PlanIt authority — postcodes.io found no nearby admin district, or no mapping
// exists for it. The watch-zone create handler surfaces it as a 500, mirroring
// .NET PostcodesIoAuthorityResolver's propagated InvalidOperationException.
var ErrAuthorityUnresolved = errors.New("authority could not be resolved from coordinates")

// reverseGeocodeResponse is the postcodes.io reverse-lookup envelope. Unlike the
// single-postcode endpoint, the reverse endpoint (/postcodes?lon=&lat=) returns
// an array of nearest matches; the first carries the admin district.
type reverseGeocodeResponse struct {
	Result []reverseGeocodeResult `json:"result"`
}

type reverseGeocodeResult struct {
	AdminDistrict *string `json:"admin_district"`
}

// ResolveAuthority reverse-geocodes coordinates to a PlanIt authority id via
// postcodes.io, mirroring .NET PostcodesIoAuthorityResolver.ResolveFromCoordinatesAsync:
// it queries the nearest postcode, reads its admin district, and looks the
// district up in the embedded mapping. A non-2xx response, an absent admin
// district, or an unmapped district all yield an error wrapping
// ErrAuthorityUnresolved (a 500 at the boundary); a transport failure is
// returned wrapped (also a 500).
func (c *Client) ResolveAuthority(ctx context.Context, latitude, longitude float64) (int, error) {
	lat := strconv.FormatFloat(latitude, 'g', -1, 64)
	lon := strconv.FormatFloat(longitude, 'g', -1, 64)
	endpoint := c.baseURL + "/postcodes?lon=" + url.QueryEscape(lon) + "&lat=" + url.QueryEscape(lat)
	// gosec G107: the host is the trusted PostcodesIoBaseURL config value; the
	// query carries only numeric coordinates, so no attacker-controlled host is
	// reachable — not an SSRF vector.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil) //nolint:gosec // trusted config host + numeric coordinates
	if err != nil {
		return 0, fmt.Errorf("build reverse-geocode request: %w", err)
	}

	resp, err := c.http.Do(req) //nolint:gosec // trusted config host, not user input
	if err != nil {
		return 0, fmt.Errorf("reverse-geocode request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("reverse geocode (%v, %v): status %d: %w", latitude, longitude, resp.StatusCode, ErrAuthorityUnresolved)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return 0, fmt.Errorf("read reverse-geocode response: %w", err)
	}
	var parsed reverseGeocodeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("decode reverse-geocode response: %w", err)
	}

	adminDistrict := firstAdminDistrict(parsed.Result)
	if adminDistrict == "" {
		return 0, fmt.Errorf("no local authority for coordinates (%v, %v): %w", latitude, longitude, ErrAuthorityUnresolved)
	}
	authorityID, ok := authorityMapping[adminDistrict]
	if !ok {
		return 0, fmt.Errorf("no PlanIt authority mapping for admin district %q: %w", adminDistrict, ErrAuthorityUnresolved)
	}
	return authorityID, nil
}

// firstAdminDistrict returns the first non-empty admin district in the result
// list, matching .NET's Result.FirstOrDefault().AdminDistrict.
func firstAdminDistrict(results []reverseGeocodeResult) string {
	for _, r := range results {
		if r.AdminDistrict != nil && *r.AdminDistrict != "" {
			return *r.AdminDistrict
		}
	}
	return ""
}
