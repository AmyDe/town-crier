// Package planit is the Go port of TownCrier.Infrastructure.PlanIt.PlanItClient:
// a rate-limited HTTP client for the PlanIt applications API. It throttles
// requests, retries idempotent GETs on transient 5xx/408 with exponential
// backoff, and surfaces HTTP 429 as a typed RateLimitError carrying the parsed
// Retry-After hint (429 is deliberately NOT retried — the poll scheduler uses
// Retry-After to choose the next cycle, and internal retries would burn the
// handler's wall-clock budget). Returned applications reuse the
// applications.PlanningApplication domain snapshot so the poll handler upserts
// them through the existing Applications store unchanged.
package planit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/polling"
)

// defaultPageSize is PlanIt's page size; a full page (>= this many records) is
// the page-fill heuristic for "more pages may follow", matching .NET.
const defaultPageSize = 100

// maxResponseBytes bounds a PlanIt JSON body so a hostile or broken upstream
// cannot exhaust memory. A 100-record page is well under this.
const maxResponseBytes = 10 << 20 // 10 MiB

// Sentinel errors for construction-time validation.
var (
	// ErrMissingBaseURL is returned when the PlanIt base URL is empty.
	ErrMissingBaseURL = errors.New("planit base URL is required")
	// ErrInsecureBaseURL is returned for a non-HTTPS base URL other than localhost.
	ErrInsecureBaseURL = errors.New("planit base URL must be https (except localhost)")
)

// RateLimitError is returned when PlanIt responds 429. RetryAfter carries the
// parsed Retry-After hint, or nil when the header was absent or malformed. The
// poll handler treats this as an expected, handled outcome (not an unhandled
// exception) and the scheduler consumes RetryAfter to pick the next cycle.
type RateLimitError struct {
	RetryAfter *time.Duration
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter != nil {
		return fmt.Sprintf("planit rate limited (retry-after %s)", *e.RetryAfter)
	}
	return "planit rate limited (no retry-after)"
}

// httpError is a non-429, non-2xx response from PlanIt that the client gave up
// on (either permanent 4xx or a 5xx that exhausted retries).
type httpError struct {
	StatusCode int
}

func (e *httpError) Error() string {
	return fmt.Sprintf("planit http error: status %d", e.StatusCode)
}

// ThrottleOptions paces outbound requests. Mirrors .NET PlanItThrottleOptions.
type ThrottleOptions struct {
	DelayBetweenRequests time.Duration
}

// RetryOptions tune the exponential-backoff retry on transient failures. Mirrors
// .NET PlanItRetryOptions.
type RetryOptions struct {
	MaxRetries       int
	InitialBackoff   time.Duration
	RateLimitBackoff time.Duration
}

// DefaultThrottleOptions returns the .NET default (2s between requests).
func DefaultThrottleOptions() ThrottleOptions {
	return ThrottleOptions{DelayBetweenRequests: 2 * time.Second}
}

// DefaultRetryOptions returns the .NET defaults (3 retries, 1s initial, 5s
// rate-limit backoff base).
func DefaultRetryOptions() RetryOptions {
	return RetryOptions{
		MaxRetries:       3,
		InitialBackoff:   1 * time.Second,
		RateLimitBackoff: 5 * time.Second,
	}
}

// Options configures a Client. HTTPClient and Sleep default to a hardened
// shared client and a context-aware time.Sleep when nil.
type Options struct {
	BaseURL    string
	Throttle   ThrottleOptions
	Retry      RetryOptions
	HTTPClient *http.Client
	// Sleep is injected so tests can pace deterministically without real waits.
	// It must honour ctx cancellation. Defaults to a context-aware sleep.
	Sleep func(ctx context.Context, d time.Duration) error
}

// FetchPageResult is one page of a PlanIt fetch: the parsed applications, the
// reported total (nil when PlanIt omitted it), and whether more pages may follow
// (the page-fill heuristic). Mirrors .NET FetchPageResult.
type FetchPageResult struct {
	Page         int
	Applications []applications.PlanningApplication
	Total        *int
	HasMorePages bool
}

// Client is the rate-limited PlanIt HTTP client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	throttle   ThrottleOptions
	retry      RetryOptions
	sleep      func(ctx context.Context, d time.Duration) error
}

// NewClient validates the base URL and wires the client. A non-HTTPS base URL is
// rejected unless it targets localhost (for tests). HTTPClient and Sleep fall
// back to hardened defaults when nil.
func NewClient(opts Options) (*Client, error) {
	if opts.BaseURL == "" {
		return nil, ErrMissingBaseURL
	}
	u, err := url.Parse(opts.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse planit base URL: %w", err)
	}
	if u.Scheme != "https" && !isLocalhost(u.Hostname()) {
		return nil, ErrInsecureBaseURL
	}

	hc := opts.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	sleep := opts.Sleep
	if sleep == nil {
		sleep = contextSleep
	}

	return &Client{
		httpClient: hc,
		baseURL:    strings.TrimRight(opts.BaseURL, "/"),
		throttle:   opts.Throttle,
		retry:      opts.Retry,
		sleep:      sleep,
	}, nil
}

// FetchApplicationsPage fetches one page of applications for authorityID, scoped
// to records changed on or after differentStart (nil for an unbounded fetch).
// It throttles, retries transient failures, and surfaces 429 as *RateLimitError.
func (c *Client) FetchApplicationsPage(ctx context.Context, authorityID int, differentStart *time.Time, page int) (FetchPageResult, error) {
	target := c.baseURL + buildPath(authorityID, differentStart, page)

	resp, err := c.sendWithThrottle(ctx, target)
	if err != nil {
		return FetchPageResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if err := classify(resp); err != nil {
		return FetchPageResult{}, err
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return FetchPageResult{}, fmt.Errorf("read planit page %d body: %w", page, err)
	}

	var parsed planItResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return FetchPageResult{}, fmt.Errorf("decode planit page %d: %w", page, err)
	}

	apps := make([]applications.PlanningApplication, 0, len(parsed.Records))
	for _, rec := range parsed.Records {
		app, err := rec.toDomain()
		if err != nil {
			return FetchPageResult{}, fmt.Errorf("map planit record %q: %w", rec.UID, err)
		}
		apps = append(apps, app)
	}

	return FetchPageResult{
		Page:         page,
		Applications: apps,
		Total:        parsed.Total,
		HasMorePages: len(apps) >= defaultPageSize,
	}, nil
}

// sendWiththrottle applies the inter-request delay, sends the GET, and retries
// transient (5xx/408) failures with exponential backoff. 429 and permanent 4xx
// are returned to the caller without retry. The returned response (when err is
// nil) has an un-read body the caller must classify and close.
func (c *Client) sendWithThrottle(ctx context.Context, target string) (*http.Response, error) {
	maxAttempts := 1 + c.retry.MaxRetries
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if c.throttle.DelayBetweenRequests > 0 {
			if err := c.sleep(ctx, c.throttle.DelayBetweenRequests); err != nil {
				return nil, err
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
		if err != nil {
			return nil, fmt.Errorf("build planit request: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Transport-level failure: retry like a transient server error.
			if attempt >= maxAttempts-1 {
				return nil, fmt.Errorf("planit request failed: %w", err)
			}
			if berr := c.sleep(ctx, c.backoff(c.retry.InitialBackoff, attempt)); berr != nil {
				return nil, berr
			}
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		// 429 and permanent (non-retryable) statuses go straight back to the
		// caller; classify() turns them into the right typed error.
		if resp.StatusCode == http.StatusTooManyRequests || !isRetryable(resp.StatusCode) {
			return resp, nil
		}

		isLast := attempt >= maxAttempts-1
		if isLast {
			return resp, nil
		}

		_ = resp.Body.Close()
		if err := c.sleep(ctx, c.backoff(c.retry.InitialBackoff, attempt)); err != nil {
			return nil, err
		}
	}
	// Unreachable: the loop always returns inside the body.
	return nil, errors.New("planit retry loop exited unexpectedly")
}

// classify maps a non-2xx response to a typed error, leaving 2xx as nil. A 429
// becomes *RateLimitError with the parsed Retry-After; any other non-2xx becomes
// *httpError.
func classify(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		var retryAfter *time.Duration
		if d, ok := polling.ParseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
			retryAfter = &d
		}
		return &RateLimitError{RetryAfter: retryAfter}
	}
	return &httpError{StatusCode: resp.StatusCode}
}

// backoff computes exponential backoff: base * 2^attempt.
func (c *Client) backoff(base time.Duration, attempt int) time.Duration {
	return base << attempt //nolint:gosec // attempt is a small bounded loop index
}

// isRetryable reports whether a status code is a transient server error worth
// retrying. 429 is intentionally excluded (handled via Retry-After).
func isRetryable(status int) bool {
	switch status {
	case http.StatusGatewayTimeout, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusRequestTimeout:
		return true
	default:
		return false
	}
}

// buildPath builds the PlanIt applications query path, mirroring .NET BuildUrl:
// pg_sz, sort=last_different, page, auth, and optional different_start (date).
func buildPath(authorityID int, differentStart *time.Time, page int) string {
	path := fmt.Sprintf("/api/applics/json?pg_sz=%d&sort=last_different&page=%d&auth=%d", defaultPageSize, page, authorityID)
	if differentStart != nil {
		path += "&different_start=" + differentStart.UTC().Format("2006-01-02")
	}
	return path
}

// isLocalhost reports whether host is a loopback name, so http is permitted in
// tests without weakening production HTTPS enforcement.
func isLocalhost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// contextSleep is the default Sleep: it waits d but returns early with the
// context error if ctx is cancelled first.
func contextSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
