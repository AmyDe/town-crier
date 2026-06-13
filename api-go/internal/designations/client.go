// Package designations owns the planning-designation feature: the GOV.UK
// planning.data.gov.uk outbound client and GET /v1/designations. It mirrors the
// .NET TownCrier.{Domain,Application,Infrastructure} Designations slices
// (GH#418 iteration 7).
package designations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// datasets is the comma-separated dataset filter sent to the entity endpoint.
// The commas are intentionally left unescaped to match the .NET client, which
// concatenates this string straight into the query.
const datasets = "conservation-area,listed-building-outline,article-4-direction-area"

// maxRespBytes bounds the planning.data.gov.uk response body read.
const maxRespBytes = 1 << 20

// Context is the planning-designation context for a point, mirroring .NET
// DesignationContext. The zero value is the "none" context (all false/nil) the
// API returns when a point intersects no designated entity.
type Context struct {
	IsWithinConservationArea        bool
	ConservationAreaName            *string
	IsWithinListedBuildingCurtilage bool
	ListedBuildingGrade             *string
	IsWithinArticle4Area            bool
}

// Client is the planning.data.gov.uk outbound designation provider. The base URL
// is config-supplied (default https://www.planning.data.gov.uk/), so no user
// input reaches the scheme.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient builds a provider over the given base URL and shared HTTP client.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: httpClient}
}

// entityResponse mirrors the GOV.UK entity.json envelope.
type entityResponse struct {
	Entities []entity `json:"entities"`
}

type entity struct {
	Dataset             string  `json:"dataset"`
	Name                string  `json:"name"`
	Reference           string  `json:"reference"`
	ListedBuildingGrade *string `json:"listed-building-grade"`
}

// Get resolves the designation context for a point. A 404 — the entity endpoint's
// response when the geometry intersects nothing, which is most UK points —
// yields the empty context with no error, mirroring .NET. Any other non-2xx, a
// transport failure, or a malformed body returns an error; the handler maps that
// to the empty context (mirroring .NET's catch of HttpRequestException).
func (c *Client) Get(ctx context.Context, latitude, longitude float64) (Context, error) {
	// WKT is "POINT(longitude latitude)" with invariant (shortest round-trip)
	// formatting, escaped exactly as .NET's Uri.EscapeDataString does.
	point := "POINT(" + strconv.FormatFloat(longitude, 'g', -1, 64) + " " + strconv.FormatFloat(latitude, 'g', -1, 64) + ")"
	endpoint := c.baseURL + "/api/v1/entity.json?geometry_intersects=" + escapeDataString(point) + "&dataset=" + datasets

	// gosec G704: the host is the trusted GovUkBaseURL config value; the query is
	// built from parsed float coordinates and a fixed dataset list, so no
	// attacker-controlled host can be reached — not an SSRF vector.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil) //nolint:gosec // trusted config host + numeric coordinates
	if err != nil {
		return Context{}, fmt.Errorf("build designations request: %w", err)
	}

	resp, err := c.http.Do(req) //nolint:gosec // trusted config host, not user input
	if err != nil {
		return Context{}, fmt.Errorf("designations request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return Context{}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Context{}, fmt.Errorf("designations request: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
	if err != nil {
		return Context{}, fmt.Errorf("read designations response: %w", err)
	}
	var parsed entityResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Context{}, fmt.Errorf("decode designations response: %w", err)
	}
	if len(parsed.Entities) == 0 {
		return Context{}, nil
	}
	return mapContext(parsed.Entities), nil
}

// mapContext folds the matched entities into a designation context, picking the
// first entity of each dataset, mirroring the .NET MapToDesignationContext.
func mapContext(entities []entity) Context {
	conservation := findDataset(entities, "conservation-area")
	listed := findDataset(entities, "listed-building-outline")
	article4 := findDataset(entities, "article-4-direction-area")

	result := Context{
		IsWithinConservationArea:        conservation != nil,
		IsWithinListedBuildingCurtilage: listed != nil,
		IsWithinArticle4Area:            article4 != nil,
	}
	if conservation != nil {
		name := conservation.Name
		result.ConservationAreaName = &name
	}
	if listed != nil {
		result.ListedBuildingGrade = listed.ListedBuildingGrade
	}
	return result
}

// findDataset returns the first entity whose dataset matches (case-insensitively),
// or nil, mirroring .NET's List.Find with OrdinalIgnoreCase.
func findDataset(entities []entity, dataset string) *entity {
	for i := range entities {
		if strings.EqualFold(entities[i].Dataset, dataset) {
			return &entities[i]
		}
	}
	return nil
}

const upperHex = "0123456789ABCDEF"

// escapeDataString percent-encodes everything outside the RFC 3986 unreserved
// set (ALPHA / DIGIT / "-" / "." / "_" / "~"), reproducing .NET's
// Uri.EscapeDataString so the geometry_intersects value is byte-identical on the
// wire — in particular a space becomes %20 (not "+", as url.QueryEscape would).
func escapeDataString(s string) string {
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if isUnreserved(c) {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(upperHex[c>>4])
		b.WriteByte(upperHex[c&0x0f])
	}
	return b.String()
}

func isUnreserved(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	case c == '-', c == '.', c == '_', c == '~':
		return true
	default:
		return false
	}
}
