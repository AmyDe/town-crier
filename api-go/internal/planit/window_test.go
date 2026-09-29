package planit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const emptyPage = `{"total":0,"pg_sz":300,"records":[]}`

type capturedRequest struct {
	rawQuery  string
	userAgent string
}

func windowServer(t *testing.T, status int, body string, hdr map[string]string) (*httptest.Server, *[]capturedRequest, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	var captured []capturedRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		captured = append(captured, capturedRequest{rawQuery: r.URL.RawQuery, userAgent: r.UserAgent()})
		for k, v := range hdr {
			w.Header().Set(k, v)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &captured, &calls
}

func newWindowClient(t *testing.T, baseURL string, mutate func(*Options)) *Client {
	t.Helper()
	opts := Options{
		BaseURL:    baseURL,
		Version:    "1.2.3",
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
	if mutate != nil {
		mutate(&opts)
	}
	c, err := NewClient(opts)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestFetchWindowPage_QueryShapes(t *testing.T) {
	t.Parallel()
	sel := selectParam(ingestSelectFields)
	tail := "&sort=last_different&pg_sz=300&index=%d&select=" + sel + "&compress=on"
	diff := day(2026, 9, 28)

	tests := []struct {
		name   string
		areaID int
		q      WindowQuery
		want   string
	}{
		{"start window", 0, WindowQuery{Axis: AxisStart, From: day(2026, 9, 20), To: day(2026, 9, 20), Index: 0},
			"start_date=2026-09-20&end_date=2026-09-20" + strings.Replace(tail, "%d", "0", 1)},
		{"decided window page 2", 0, WindowQuery{Axis: AxisDecided, From: day(2026, 9, 20), To: day(2026, 9, 20), Index: 600},
			"decided_start=2026-09-20&decided_end=2026-09-20" + strings.Replace(tail, "%d", "600", 1)},
		{"start delta", 0, WindowQuery{Axis: AxisStart, From: day(2026, 9, 15), To: day(2026, 9, 29), DifferentStart: &diff},
			"different_start=2026-09-28&start_date=2026-09-15" + strings.Replace(tail, "%d", "0", 1)},
		{"decided delta", 0, WindowQuery{Axis: AxisDecided, From: day(2026, 9, 15), To: day(2026, 9, 29), DifferentStart: &diff},
			"different_start=2026-09-28&decided_start=2026-09-15" + strings.Replace(tail, "%d", "0", 1)},
		{"oracle start wide read with auth", 42, WindowQuery{Axis: AxisStart, From: day(2026, 7, 1), To: day(2026, 9, 29)},
			"start_date=2026-07-01&end_date=2026-09-29" + strings.Replace(tail, "%d", "0", 1) + "&auth=42"},
		{"oracle decided wide read with auth", 42, WindowQuery{Axis: AxisDecided, From: day(2026, 7, 1), To: day(2026, 9, 29)},
			"decided_start=2026-07-01&decided_end=2026-09-29" + strings.Replace(tail, "%d", "0", 1) + "&auth=42"},
		{"window with auth", 7, WindowQuery{Axis: AxisStart, From: day(2026, 9, 20), To: day(2026, 9, 20)},
			"start_date=2026-09-20&end_date=2026-09-20" + strings.Replace(tail, "%d", "0", 1) + "&auth=7"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, got, _ := windowServer(t, 200, emptyPage, nil)
			c := newWindowClient(t, srv.URL, func(o *Options) { o.AreaID = tc.areaID })
			if _, err := c.FetchWindowPage(context.Background(), tc.q); err != nil {
				t.Fatalf("FetchWindowPage: %v", err)
			}
			if len(*got) != 1 {
				t.Fatalf("requests: got %d, want 1", len(*got))
			}
			if (*got)[0].rawQuery != tc.want {
				t.Errorf("query:\n got %s\nwant %s", (*got)[0].rawQuery, tc.want)
			}
			if tc.areaID == 0 && strings.Contains((*got)[0].rawQuery, "auth=") {
				t.Errorf("auth present without AreaID: %s", (*got)[0].rawQuery)
			}
		})
	}
}

func TestFetchWindowPage_SendsUserAgent(t *testing.T) {
	t.Parallel()
	srv, got, _ := windowServer(t, 200, emptyPage, nil)
	c := newWindowClient(t, srv.URL, nil)
	if _, err := c.FetchWindowPage(context.Background(), WindowQuery{Axis: AxisStart, From: day(2026, 9, 20), To: day(2026, 9, 20)}); err != nil {
		t.Fatalf("FetchWindowPage: %v", err)
	}
	want := "TownCrier/1.2.3 (+https://towncrierapp.uk; support@towncrierapp.uk)"
	if (*got)[0].userAgent != want {
		t.Errorf("User-Agent: got %q, want %q", (*got)[0].userAgent, want)
	}
}

func TestFetchWindowPage_EmptyVersionUsesDev(t *testing.T) {
	t.Parallel()
	srv, got, _ := windowServer(t, 200, emptyPage, nil)
	c := newWindowClient(t, srv.URL, func(o *Options) { o.Version = "" })
	if _, err := c.FetchWindowPage(context.Background(), WindowQuery{Axis: AxisStart, From: day(2026, 9, 20), To: day(2026, 9, 20)}); err != nil {
		t.Fatalf("FetchWindowPage: %v", err)
	}
	if !strings.HasPrefix((*got)[0].userAgent, "TownCrier/dev (") {
		t.Errorf("User-Agent: got %q", (*got)[0].userAgent)
	}
}

func TestFetchWindowPage_ParsesTotalAndHasMore(t *testing.T) {
	t.Parallel()
	srv, _, _ := windowServer(t, 200, `{"total":301,"from":0,"pg_sz":300,"records":[]}`, nil)
	c := newWindowClient(t, srv.URL, nil)
	_, err := c.FetchWindowPage(context.Background(), WindowQuery{Axis: AxisStart, From: day(2026, 9, 20), To: day(2026, 9, 20)})
	if !errors.Is(err, ErrZeroProgress) {
		t.Fatalf("zero records below total should be ErrZeroProgress, got %v", err)
	}
}

func TestFetchWindowPage_ErrorMappingAndNoRetry(t *testing.T) {
	t.Parallel()
	q := WindowQuery{Axis: AxisStart, From: day(2026, 9, 20), To: day(2026, 9, 20)}

	t.Run("429 with Retry-After", func(t *testing.T) {
		t.Parallel()
		srv, _, calls := windowServer(t, 429, "", map[string]string{"Retry-After": "120"})
		c := newWindowClient(t, srv.URL, nil)
		_, err := c.FetchWindowPage(context.Background(), q)
		var rl *RateLimitError
		if !errors.As(err, &rl) {
			t.Fatalf("want *RateLimitError, got %v", err)
		}
		if rl.RetryAfter == nil || *rl.RetryAfter != 120*time.Second {
			t.Errorf("RetryAfter: %v", rl.RetryAfter)
		}
		if calls.Load() != 1 {
			t.Errorf("calls: %d, want 1", calls.Load())
		}
	})
	t.Run("403", func(t *testing.T) {
		t.Parallel()
		srv, _, calls := windowServer(t, 403, "blocked", nil)
		c := newWindowClient(t, srv.URL, nil)
		_, err := c.FetchWindowPage(context.Background(), q)
		var fe *ForbiddenError
		if !errors.As(err, &fe) {
			t.Fatalf("want *ForbiddenError, got %v", err)
		}
		if calls.Load() != 1 {
			t.Errorf("calls: %d, want 1", calls.Load())
		}
	})
	t.Run("503 is not retried", func(t *testing.T) {
		t.Parallel()
		srv, _, calls := windowServer(t, 503, "down", nil)
		c := newWindowClient(t, srv.URL, nil)
		_, err := c.FetchWindowPage(context.Background(), q)
		var he *HTTPError
		if !errors.As(err, &he) || he.StatusCode != 503 {
			t.Fatalf("want *HTTPError 503, got %v", err)
		}
		if calls.Load() != 1 {
			t.Errorf("calls: %d, want 1", calls.Load())
		}
	})
	t.Run("timeout", func(t *testing.T) {
		t.Parallel()
		release := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
		t.Cleanup(func() { close(release); srv.Close() })
		c := newWindowClient(t, srv.URL, func(o *Options) {
			o.HTTPClient = &http.Client{Timeout: 50 * time.Millisecond}
		})
		_, err := c.FetchWindowPage(context.Background(), q)
		if !errors.Is(err, ErrTimeout) {
			t.Fatalf("want ErrTimeout, got %v", err)
		}
	})
	t.Run("caller cancellation is not a timeout", func(t *testing.T) {
		t.Parallel()
		srv, _, _ := windowServer(t, 200, emptyPage, nil)
		c := newWindowClient(t, srv.URL, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := c.FetchWindowPage(ctx, q)
		if !errors.Is(err, context.Canceled) || errors.Is(err, ErrTimeout) {
			t.Fatalf("want context.Canceled only, got %v", err)
		}
	})
}

func TestFetchWindowPage_SpanAttributes(t *testing.T) {
	t.Parallel()
	srv, _, _ := windowServer(t, 200, emptyPage, nil)
	tp, rec := recorderProvider(t)
	c := newWindowClient(t, srv.URL, func(o *Options) {
		o.TraceOptions = []otelhttp.Option{otelhttp.WithTracerProvider(tp)}
	})
	_, err := c.FetchWindowPage(context.Background(), WindowQuery{
		Work: WorkWindowStart, Axis: AxisStart, From: day(2026, 9, 20), To: day(2026, 9, 20),
	})
	if err != nil {
		t.Fatalf("FetchWindowPage: %v", err)
	}
	spans := rec.Ended()
	if len(spans) != 1 || spans[0].Name() != "PlanIt search" {
		t.Fatalf("spans: %v", spans)
	}
	if v, ok := attr(spans[0].Attributes(), "planit.work"); !ok || v.AsString() != "window_start" {
		t.Errorf("planit.work: %v %v", v, ok)
	}
	if v, ok := attr(spans[0].Attributes(), "planit.window_day"); !ok || v.AsString() != "2026-09-20" {
		t.Errorf("planit.window_day: %v %v", v, ok)
	}
}

func TestFetchWindowPage_DeltaSpanHasNoWindowDay(t *testing.T) {
	t.Parallel()
	srv, _, _ := windowServer(t, 200, emptyPage, nil)
	tp, rec := recorderProvider(t)
	c := newWindowClient(t, srv.URL, func(o *Options) {
		o.TraceOptions = []otelhttp.Option{otelhttp.WithTracerProvider(tp)}
	})
	diff := day(2026, 9, 28)
	_, err := c.FetchWindowPage(context.Background(), WindowQuery{
		Work: WorkDeltaStart, Axis: AxisStart, From: day(2026, 9, 15), To: day(2026, 9, 29), DifferentStart: &diff,
	})
	if err != nil {
		t.Fatalf("FetchWindowPage: %v", err)
	}
	attrs := rec.Ended()[0].Attributes()
	if v, ok := attr(attrs, "planit.work"); !ok || v.AsString() != "delta_start" {
		t.Errorf("planit.work: %v %v", v, ok)
	}
	if _, ok := attr(attrs, "planit.window_day"); ok {
		t.Errorf("delta span must not carry planit.window_day")
	}
}
