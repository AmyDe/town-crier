// Package planit is the HTTP client for the PlanIt applications API. One call
// is one request: no throttling and no retries (the polling Pacer owns both).
// 429 surfaces as *RateLimitError, 403 as *ForbiddenError and a timeout as
// ErrTimeout. Returned applications are applications.PlanningApplication.
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

// maxResponseBytes bounds a PlanIt JSON body so a hostile or broken upstream
// cannot exhaust memory. A 300-record page is well under this.
const maxResponseBytes = 10 << 20 // 10 MiB

// errorBodyPrefixBytes bounds the error body classify captures onto HTTPError
// for a non-2xx, non-429 response: enough for PlanIt's typical
// {"error": "..."} shape, small enough to be a safe OTel span attribute.
const errorBodyPrefixBytes = 512

// Sentinel errors for construction-time validation.
var (
	// ErrMissingBaseURL is returned when the PlanIt base URL is empty.
	ErrMissingBaseURL = errors.New("planit base URL is required")
	// ErrInsecureBaseURL is returned for a non-HTTPS base URL other than localhost.
	ErrInsecureBaseURL = errors.New("planit base URL must be https (except localhost)")
)

// ErrZeroProgress is returned when PlanIt responds 200 with zero records while
// its own total reports more records remain (from < total). Without this guard
// a caller that resumes at the same index would loop forever.
var ErrZeroProgress = errors.New("planit: zero-progress response (0 records, from < total)")

// RateLimitError is returned when PlanIt responds 429. RetryAfter carries the
// parsed Retry-After hint, or nil when the header was absent or malformed.
type RateLimitError struct {
	RetryAfter *time.Duration
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter != nil {
		return fmt.Sprintf("planit rate limited (retry-after %s)", *e.RetryAfter)
	}
	return "planit rate limited (no retry-after)"
}

// HTTPError is a non-429, non-403, non-2xx response from PlanIt. Body carries a
// bounded prefix (errorBodyPrefixBytes) of the response body: for a 400 this is
// usually the failure reason, such as a bad column name in the query.
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

// httpErrorRecorder is the consumer-side slice of the metrics registry the
// client records towncrier.planit.http_errors on. *metrics.Registry satisfies
// it; nil leaves the counter dark. The tag keys are owned by the recorder
// (http.response.status_code, planit.authority_code).
type httpErrorRecorder interface {
	PlanItHTTPError(ctx context.Context, statusCode, authorityID int)
}

// Options configures a Client. HTTPClient defaults to a client with a 30 second
// timeout when nil.
type Options struct {
	BaseURL    string
	HTTPClient *http.Client
	// Metrics records towncrier.planit.http_errors. nil leaves the counter dark.
	Metrics httpErrorRecorder
	// TraceOptions are extra otelhttp options threaded into the wrapped transport
	// (e.g. WithTracerProvider in hermetic tests).
	TraceOptions []otelhttp.Option
	// AreaID, when non-zero, adds &auth=<AreaID> to every FetchWindowPage query.
	AreaID int
	// Version is the build version sent in the User-Agent. Empty sends "dev".
	Version string
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

// Client is the PlanIt HTTP client. It makes one request per call and never
// retries or paces: the polling Pacer owns both.
type Client struct {
	httpClient *http.Client
	baseURL    string
	metrics    httpErrorRecorder
	areaID     int
	userAgent  string
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
	inner := *hc
	inner.Transport = spanAttrTransport{next: hc.Transport}
	hc = platform.WrapHTTPClient(&inner, func(string, *http.Request) string { return "PlanIt search" }, opts.TraceOptions...)

	return &Client{
		httpClient: hc,
		baseURL:    strings.TrimRight(opts.BaseURL, "/"),
		metrics:    opts.Metrics,
		areaID:     opts.AreaID,
		userAgent:  userAgent(opts.Version),
	}, nil
}

// decodePage classifies resp, decodes the PlanIt envelope, and maps its
// records. It closes resp.Body.
func decodePage(resp *http.Response, authorityIDForMetrics, startIndex, pageSize int) (FetchPageResult, error) {
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

// classify maps a non-2xx response to a typed error, leaving 2xx as nil. A 429
// becomes *RateLimitError with the parsed Retry-After. Any other non-2xx
// becomes *HTTPError carrying a bounded prefix of the response body.
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
	// Best-effort: a read failure here still leaves a valid HTTPError with
	// StatusCode set (io.ReadAll returns whatever partial bytes it read even
	// on error), so there's nothing actionable to do with the error itself.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, errorBodyPrefixBytes)) //nolint:errcheck // partial body is fine; StatusCode is what matters
	return &HTTPError{StatusCode: resp.StatusCode, Body: string(body)}
}

// isLocalhost reports whether host is a loopback name, so http is permitted in
// tests without weakening production HTTPS enforcement.
func isLocalhost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
