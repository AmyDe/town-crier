package tc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// apiKeyHeader is the admin authentication header the API expects.
const apiKeyHeader = "X-Admin-Key"

// maxRespBytes bounds JSON response bodies read fully into memory.
const maxRespBytes = 10 << 20 // 10 MiB

// Client is a thin admin-API HTTP client. It targets the configured base URL and
// attaches the X-Admin-Key header to every request. It does not enforce HTTPS,
// so http://localhost dev endpoints work unchanged.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// NewClient builds a Client from resolved config. The trailing slash on the base
// URL is trimmed so path joins are predictable.
func NewClient(cfg Config) *Client {
	return &Client{
		baseURL: strings.TrimRight(cfg.URL, "/"),
		apiKey:  cfg.APIKey,
		http:    &http.Client{Timeout: 100 * time.Second},
	}
}

// do builds and sends a request with the given method, path, and optional JSON
// body, returning the raw response for the caller to interpret.
func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set(apiKeyHeader, c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.http.Do(req)
}

// Post sends a JSON POST and returns the raw response.
func (c *Client) Post(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.do(ctx, http.MethodPost, path, body)
}

// Put sends a JSON PUT and returns the raw response.
func (c *Client) Put(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.do(ctx, http.MethodPut, path, body)
}

// GetJSON sends a GET and decodes a successful JSON response into out. A non-2xx
// status becomes an error of the form "API error (<status>): <body>".
func (c *Client) GetJSON(ctx context.Context, path string, out any) error {
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxRespBytes))
		return fmt.Errorf("API error (%d): %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(io.LimitReader(resp.Body, maxRespBytes)).Decode(out)
}
