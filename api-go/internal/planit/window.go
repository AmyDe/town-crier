package planit

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// nationalPageSize is the pg_sz of every window query. It is a fixed safety
// rule, not a tunable: the PlanIt operator asked for modest page sizes.
const nationalPageSize = 300

// ingestSelectFields lists every field the ingest pipeline consumes.
// last_different must stay: PlanIt returns 400 if the sort field is not selected.
var ingestSelectFields = []string{
	"name", "uid", "area_name", "area_id", "address", "postcode", "description",
	"app_type", "app_state", "app_size", "start_date", "decided_date", "consulted_date",
	"location_x", "location_y", "url", "link", "last_different", "reference", "altid",
	"associated_id", "last_changed", "last_scraped", "scraper_name", "other_fields",
}

func selectParam(fields []string) string {
	return strings.Join(fields, ",")
}

// contactUserAgentSuffix identifies Town Crier to PlanIt's operator (POLLING.md
// "User-Agent"). The address is deliberately a constant, never configuration.
const contactUserAgentSuffix = "(+https://towncrierapp.uk; support@towncrierapp.uk)"

// Axis selects which date field a window query filters on.
type Axis int

// The two window axes.
const (
	AxisStart Axis = iota
	AxisDecided
)

// Work labels why a query is made; it is recorded as the planit.work span attribute.
type Work string

// Work values recorded on the PlanIt search span.
const (
	WorkWindowStart   Work = "window_start"
	WorkWindowDecided Work = "window_decided"
	WorkDeltaStart    Work = "delta_start"
	WorkDeltaDecided  Work = "delta_decided"
	WorkOracleStart   Work = "oracle_start"
	WorkOracleDecided Work = "oracle_decided"
)

// ErrTimeout is returned when PlanIt does not respond within the client's HTTP
// timeout. A caller-cancelled context is not a timeout.
var ErrTimeout = errors.New("planit: request timed out")

// ForbiddenError is returned when PlanIt responds 403, which it uses to block
// clients with a missing or invalid User-Agent or excessive volume.
type ForbiddenError struct{}

func (*ForbiddenError) Error() string { return "planit forbidden (403)" }

// WindowQuery describes one page of a date-window or delta read. From and To
// are inclusive dates on Axis. When DifferentStart is set the query is a delta:
// only From is sent, as the axis mask, alongside different_start.
type WindowQuery struct {
	Work           Work
	Axis           Axis
	From, To       time.Time
	DifferentStart *time.Time
	Index          int
}

// FetchWindowPage fetches one page of the query. One call is exactly one HTTP
// request: there is no throttle and no retry. A 429 returns *RateLimitError,
// a 403 returns *ForbiddenError, and no response within the HTTP timeout
// returns an error matching ErrTimeout.
func (c *Client) FetchWindowPage(ctx context.Context, q WindowQuery) (FetchPageResult, error) {
	target := c.baseURL + c.buildWindowPath(q)

	if q.Work != "" {
		ctx = withSpanAttrs(ctx, windowSpanAttrs(q))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return FetchPageResult{}, fmt.Errorf("build planit request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return FetchPageResult{}, mapTransportError(ctx, err)
	}
	if resp.StatusCode != http.StatusOK && c.metrics != nil {
		c.metrics.PlanItHTTPError(ctx, resp.StatusCode, c.areaID)
	}
	if resp.StatusCode == http.StatusForbidden {
		_ = resp.Body.Close()
		return FetchPageResult{}, &ForbiddenError{}
	}
	return decodePage(resp, c.areaID, q.Index, nationalPageSize)
}

func (c *Client) buildWindowPath(q WindowQuery) string {
	const layout = "2006-01-02"
	var filter string
	switch {
	case q.DifferentStart != nil && q.Axis == AxisDecided:
		filter = fmt.Sprintf("different_start=%s&decided_start=%s", q.DifferentStart.UTC().Format(layout), q.From.UTC().Format(layout))
	case q.DifferentStart != nil:
		filter = fmt.Sprintf("different_start=%s&start_date=%s", q.DifferentStart.UTC().Format(layout), q.From.UTC().Format(layout))
	case q.Axis == AxisDecided:
		filter = fmt.Sprintf("decided_start=%s&decided_end=%s", q.From.UTC().Format(layout), q.To.UTC().Format(layout))
	default:
		filter = fmt.Sprintf("start_date=%s&end_date=%s", q.From.UTC().Format(layout), q.To.UTC().Format(layout))
	}
	path := fmt.Sprintf("/api/applics/json?%s&sort=last_different&pg_sz=%d&index=%d&select=%s&compress=on",
		filter, nationalPageSize, q.Index, selectParam(ingestSelectFields))
	if c.areaID != 0 {
		path += fmt.Sprintf("&auth=%d", c.areaID)
	}
	return path
}

func userAgent(version string) string {
	if version == "" {
		version = "dev"
	}
	return "TownCrier/" + version + " " + contactUserAgentSuffix
}

func mapTransportError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("planit request: %w", ctx.Err())
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return fmt.Errorf("%w: %w", ErrTimeout, err)
	}
	return fmt.Errorf("planit request failed: %w", err)
}

type spanAttrsKey struct{}

func withSpanAttrs(ctx context.Context, attrs []attribute.KeyValue) context.Context {
	return context.WithValue(ctx, spanAttrsKey{}, attrs)
}

func windowSpanAttrs(q WindowQuery) []attribute.KeyValue {
	attrs := []attribute.KeyValue{attribute.String("planit.work", string(q.Work))}
	if q.Work == WorkWindowStart || q.Work == WorkWindowDecided {
		attrs = append(attrs, attribute.String("planit.window_day", q.From.UTC().Format("2006-01-02")))
	}
	return attrs
}

// spanAttrTransport sits inside the otelhttp transport so that the span it
// started (already in the request context) can receive per-request attributes,
// which otelhttp itself only supports statically.
type spanAttrTransport struct {
	next http.RoundTripper
}

func (t spanAttrTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if attrs, ok := req.Context().Value(spanAttrsKey{}).([]attribute.KeyValue); ok {
		trace.SpanFromContext(req.Context()).SetAttributes(attrs...)
	}
	next := t.next
	if next == nil {
		next = http.DefaultTransport
	}
	return next.RoundTrip(req)
}
