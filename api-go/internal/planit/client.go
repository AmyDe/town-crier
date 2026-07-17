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

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// defaultPageSize is the page size used when Options.PageSize is zero. It also
// backstops the page-fill heuristic ("more pages may follow") for a response
// that omits total.
const defaultPageSize = 100

// maxResponseBytes bounds a PlanIt JSON body so a hostile or broken upstream
// cannot exhaust memory. A 100-record page is well under this.
const maxResponseBytes = 10 << 20 // 10 MiB

// errorBodyPrefixBytes bounds the error body classify captures onto HTTPError
// for a non-2xx, non-429 response: enough for PlanIt's typical
// {"error": "..."} shape, small enough to be a safe OTel span attribute
// (tc-tuge8/GH#971).
const errorBodyPrefixBytes = 512

// Sentinel errors for construction-time validation.
var (
	// ErrMissingBaseURL is returned when the PlanIt base URL is empty.
	ErrMissingBaseURL = errors.New("planit base URL is required")
	// ErrInsecureBaseURL is returned for a non-HTTPS base URL other than localhost.
	ErrInsecureBaseURL = errors.New("planit base URL must be https (except localhost)")
)

// ErrZeroProgress is returned when PlanIt responds 200 with zero records while
// its own total reports more records remain (from < total). A well-behaved
// upstream never does this; without this guard a caller that blindly retries
// the same index would livelock forever. Treated like any other authority
// fetch error by the poll handler: the authority is skipped for this cycle.
var ErrZeroProgress = errors.New("planit: zero-progress response (0 records, from < total)")

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

// HTTPError is a non-429, non-2xx response from PlanIt that the client gave up
// on (either permanent 4xx or a 5xx that exhausted retries). Body carries a
// bounded prefix (errorBodyPrefixBytes) of PlanIt's response body: for a 400
// this is usually the actual failure reason (e.g. a bad column name in the
// query), which the client previously discarded entirely (tc-tuge8/GH#971).
// Exported so callers (Lane C's reconciliation sweep) can errors.As into it
// and surface Body on their own telemetry.
type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("planit http error: status %d: %s", e.StatusCode, e.Body)
	}
	return fmt.Sprintf("planit http error: status %d", e.StatusCode)
}

// ThrottleOptions paces outbound requests.
type ThrottleOptions struct {
	DelayBetweenRequests time.Duration
}

// RetryOptions tune the exponential-backoff retry on transient failures.
type RetryOptions struct {
	MaxRetries       int
	InitialBackoff   time.Duration
	RateLimitBackoff time.Duration
}

// httpErrorRecorder is the consumer-side slice of the metrics registry the
// client records towncrier.planit.http_errors on. *metrics.Registry satisfies
// it; nil leaves the counter dark. The tag keys are owned by the recorder
// (http.response.status_code, planit.authority_code).
type httpErrorRecorder interface {
	PlanItHTTPError(ctx context.Context, statusCode, authorityID int)
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
	// Metrics records towncrier.planit.http_errors. nil leaves the counter dark
	// (the default for the many client tests that don't assert on metrics).
	Metrics httpErrorRecorder
	// PageSize is the pg_sz sent on every request and the length-heuristic
	// fallback for HasMorePages when a response omits total. Zero defaults to
	// defaultPageSize (100) — the current, unchanged behaviour. Config knob:
	// POLLING_PLANIT_PAGE_SIZE (internal/platform.Config.PollingPlanItPageSize).
	PageSize int
	// TraceOptions are extra otelhttp options threaded into the wrapped transport
	// (e.g. WithTracerProvider in hermetic tests). Production leaves this nil and
	// relies on the global provider installed by SetupTelemetry.
	TraceOptions []otelhttp.Option
}

// FetchPageResult is one fetch of a PlanIt index-paginated response: the parsed
// applications, the echoed from (the record offset the response actually
// started at), the reported total (nil when PlanIt omitted it), and whether
// more records may follow.
type FetchPageResult struct {
	// From is the record offset PlanIt's response reports it started at (the
	// response's own "from" field), falling back to the requested startIndex
	// when the response omits it. Callers use From + len(Applications) as the
	// next fetch's startIndex, so a truncated page still advances correctly.
	From         int
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
	metrics    httpErrorRecorder
	pageSize   int
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
	// Wrap the transport so every PlanIt GET emits an OTel client span
	// (Type=HTTP in AppDependencies) named "PlanIt search". The host lands in
	// server.address; the static span name keeps cardinality low.
	hc = platform.WrapHTTPClient(hc, func(string, *http.Request) string { return "PlanIt search" }, opts.TraceOptions...)
	sleep := opts.Sleep
	if sleep == nil {
		sleep = contextSleep
	}
	pageSize := opts.PageSize
	if pageSize == 0 {
		pageSize = defaultPageSize
	}

	return &Client{
		httpClient: hc,
		baseURL:    strings.TrimRight(opts.BaseURL, "/"),
		throttle:   opts.Throttle,
		retry:      opts.Retry,
		sleep:      sleep,
		metrics:    opts.Metrics,
		pageSize:   pageSize,
	}, nil
}

// FetchApplicationsPage fetches one index-paginated page of applications for
// authorityID, scoped to records changed on or after differentStart (nil for an
// unbounded fetch), starting at the 0-based record offset startIndex. descending
// selects sort=-last_different (newest first, used by the freshness probe)
// instead of the drain's default ascending sort=last_different. It throttles,
// retries transient failures, and surfaces 429 as *RateLimitError.
func (c *Client) FetchApplicationsPage(ctx context.Context, authorityID int, differentStart *time.Time, startIndex int, descending bool) (FetchPageResult, error) {
	target := c.baseURL + buildPath(authorityID, differentStart, startIndex, descending, c.pageSize)
	return c.fetchPage(ctx, target, authorityID, startIndex, c.pageSize)
}

// fetchPage sends target, classifies the response, decodes the PlanIt
// envelope, and maps its records to the domain snapshot. It is the shared tail
// end of every fetch method — the per-authority drain (FetchApplicationsPage),
// the ADR 0041 national delta lanes (FetchNationalDeltaPage), Lane C's light
// per-authority sweep (FetchReconciliationPage), and its single-uid hydration
// lookup (FetchByUID) — so the classify/decode/zero-progress logic lives in
// exactly one place. authorityIDForMetrics tags the http-error counter only; a
// national or uid-scoped request has no single owning authority, so callers
// pass 0. pageSize drives both the zero-progress guard's "from < total"
// comparison context and the length-heuristic HasMorePages fallback for a
// response that omits total.
func (c *Client) fetchPage(ctx context.Context, target string, authorityIDForMetrics, startIndex, pageSize int) (FetchPageResult, error) {
	resp, err := c.sendWithThrottle(ctx, target, authorityIDForMetrics)
	if err != nil {
		return FetchPageResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if err := classify(resp); err != nil {
		return FetchPageResult{}, err
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return FetchPageResult{}, fmt.Errorf("read planit index %d body: %w", startIndex, err)
	}

	var parsed planItResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return FetchPageResult{}, fmt.Errorf("decode planit index %d: %w", startIndex, err)
	}

	apps := make([]applications.PlanningApplication, 0, len(parsed.Records))
	for _, rec := range parsed.Records {
		app, err := rec.toDomain()
		if err != nil {
			return FetchPageResult{}, fmt.Errorf("map planit record %q: %w", rec.UID, err)
		}
		apps = append(apps, app)
	}

	from := startIndex
	if parsed.From != nil {
		from = *parsed.From
	}

	// Zero-progress guard: PlanIt reporting more records remain (from < total)
	// while returning none this fetch would livelock a caller that blindly
	// resumes at the same index forever. Treat it as a hard fetch error
	// instead — every caller's existing per-unit (authority/lane) error path
	// skips to the next unit and retries this one next cycle.
	if len(apps) == 0 && parsed.Total != nil && from < *parsed.Total {
		return FetchPageResult{}, fmt.Errorf("planit fetch (authority %d) at index %d: %w", authorityIDForMetrics, startIndex, ErrZeroProgress)
	}

	hasMorePages := len(apps) >= pageSize
	if parsed.Total != nil {
		hasMorePages = from+len(apps) < *parsed.Total
	}

	return FetchPageResult{
		From:         from,
		Applications: apps,
		Total:        parsed.Total,
		HasMorePages: hasMorePages,
	}, nil
}

// sendWiththrottle applies the inter-request delay, sends the GET, and retries
// transient (5xx/408) failures with exponential backoff. 429 and permanent 4xx
// are returned to the caller without retry. The returned response (when err is
// nil) has an un-read body the caller must classify and close.
func (c *Client) sendWithThrottle(ctx context.Context, target string, authorityID int) (*http.Response, error) {
	maxAttempts := 1 + c.retry.MaxRetries
	for attempt := range maxAttempts {
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

		// Every non-2xx response is an http error — count it on each attempt
		// (including 429 and retried 5xx), tagged with status + authority.
		if c.metrics != nil {
			c.metrics.PlanItHTTPError(ctx, resp.StatusCode, authorityID)
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
// becomes *RateLimitError with the parsed Retry-After -- its body is never
// read, matching the pre-tc-tuge8 behaviour exactly. Any other non-2xx
// becomes *HTTPError carrying a bounded prefix of the response body: resp.Body
// is guaranteed unread at this point (sendWithThrottle returns it unread on
// every non-2xx exit, and fetchPage's own io.ReadAll only runs on the nil-err
// 2xx path), so this is the one safe place to read it.
func classify(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		var retryAfter *time.Duration
		if d, ok := ParseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
			retryAfter = &d
		}
		return &RateLimitError{RetryAfter: retryAfter}
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyPrefixBytes))
	return &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
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

// buildPath builds the PlanIt applications query path: pg_sz, sort (ascending
// last_different, or descending -last_different when descending is true),
// index (the 0-based record offset), auth, and optional different_start (date).
// index= replaces the old page= parameter: a record-level resume is immune to
// PlanIt's 1MB response-body truncation, where a shortened page misread as
// end-of-results under page= silently restarted the window (GH#955).
func buildPath(authorityID int, differentStart *time.Time, startIndex int, descending bool, pageSize int) string {
	sortParam := "last_different"
	if descending {
		sortParam = "-last_different"
	}
	path := fmt.Sprintf("/api/applics/json?pg_sz=%d&sort=%s&index=%d&auth=%d", pageSize, sortParam, startIndex, authorityID)
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
