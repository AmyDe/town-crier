package planit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClock records the durations it is asked to sleep so tests can assert
// throttle/backoff behaviour without real wall-clock waits. It honours ctx
// cancellation like the real delay would.
type fakeClock struct {
	mu     sync.Mutex
	sleeps []time.Duration
}

func (c *fakeClock) sleep(ctx context.Context, d time.Duration) error {
	c.mu.Lock()
	c.sleeps = append(c.sleeps, d)
	c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (c *fakeClock) total() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	var sum time.Duration
	for _, d := range c.sleeps {
		sum += d
	}
	return sum
}

func newTestClient(t *testing.T, baseURL string, clock *fakeClock) *Client {
	t.Helper()
	c, err := NewClient(Options{
		BaseURL:    baseURL,
		Throttle:   ThrottleOptions{DelayBetweenRequests: 0},
		Retry:      RetryOptions{MaxRetries: 3, InitialBackoff: time.Second, RateLimitBackoff: 5 * time.Second},
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		Sleep:      clock.sleep,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestFetchApplicationsPage_ParsesRecordsAndTotal(t *testing.T) {
	t.Parallel()
	body := `{"total":42,"pg_sz":100,"records":[
		{"name":"24/0001","uid":"24/0001/FUL","area_name":"Test","area_id":99,"address":"1 High St","postcode":"AB1 2CD","description":"A shed","app_type":"Full","app_state":"Undecided","app_size":"Small","start_date":"2026-06-01","location_x":-0.1,"location_y":51.5,"url":"http://x","link":"http://y","last_different":"2026-06-10T09:00:00Z"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the query the .NET client builds: pg_sz, sort, page, auth, different_start.
		q := r.URL.Query()
		if q.Get("auth") != "99" || q.Get("page") != "1" || q.Get("different_start") != "2026-06-09" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	clock := &fakeClock{}
	c := newTestClient(t, srv.URL, clock)

	ds := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	res, err := c.FetchApplicationsPage(context.Background(), 99, &ds, 1)
	if err != nil {
		t.Fatalf("FetchApplicationsPage: %v", err)
	}
	if res.Total == nil || *res.Total != 42 {
		t.Errorf("total: got %v, want 42", res.Total)
	}
	if len(res.Applications) != 1 {
		t.Fatalf("applications: got %d, want 1", len(res.Applications))
	}
	app := res.Applications[0]
	if app.Name != "24/0001" || app.AreaID != 99 || app.UID != "24/0001/FUL" {
		t.Errorf("mapped app fields wrong: %+v", app)
	}
	if app.Latitude == nil || *app.Latitude != 51.5 || app.Longitude == nil || *app.Longitude != -0.1 {
		t.Errorf("coords: lat=%v lng=%v", app.Latitude, app.Longitude)
	}
	if !app.LastDifferent.Equal(time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("lastDifferent: got %v", app.LastDifferent)
	}
	// A short (< page size) record set means no more pages.
	if res.HasMorePages {
		t.Error("HasMorePages should be false for a partial page")
	}
}

func TestFetchApplicationsPage_FullPageMeansMorePages(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	sb.WriteString(`{"total":250,"records":[`)
	for i := range 100 {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"name":"r","uid":"u","area_name":"a","area_id":1,"address":"x","description":"d","app_type":"t","app_state":"s","last_different":"2026-06-10T09:00:00Z"}`)
	}
	sb.WriteString(`]}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(sb.String()))
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv.URL, &fakeClock{})
	res, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1)
	if err != nil {
		t.Fatalf("FetchApplicationsPage: %v", err)
	}
	if !res.HasMorePages {
		t.Error("HasMorePages should be true for a full page")
	}
}

func TestFetchApplicationsPage_429SurfacesRateLimitWithRetryAfterAndIsNotRetried(t *testing.T) {
	t.Parallel()
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv.URL, &fakeClock{})
	_, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1)

	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected RateLimitError, got %v", err)
	}
	if rl.RetryAfter == nil || *rl.RetryAfter != 120*time.Second {
		t.Errorf("RetryAfter: got %v, want 120s", rl.RetryAfter)
	}
	// 429 must NOT be retried (the scheduler uses Retry-After to pick next run).
	if calls != 1 {
		t.Errorf("429 should not be retried: got %d calls, want 1", calls)
	}
}

func TestFetchApplicationsPage_429WithoutHeaderHasNilRetryAfter(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv.URL, &fakeClock{})
	_, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1)

	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected RateLimitError, got %v", err)
	}
	if rl.RetryAfter != nil {
		t.Errorf("RetryAfter should be nil when header absent, got %v", rl.RetryAfter)
	}
}

func TestFetchApplicationsPage_RetriesOn503ThenSucceeds(t *testing.T) {
	t.Parallel()
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"total":0,"records":[]}`))
	}))
	t.Cleanup(srv.Close)

	clock := &fakeClock{}
	c := newTestClient(t, srv.URL, clock)
	res, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1)
	if err != nil {
		t.Fatalf("FetchApplicationsPage: %v", err)
	}
	if len(res.Applications) != 0 {
		t.Errorf("applications: got %d, want 0", len(res.Applications))
	}
	if calls != 3 {
		t.Errorf("calls: got %d, want 3 (two retries)", calls)
	}
	// Exponential backoff from InitialBackoff (1s): first retry 1s, second 2s.
	if got := clock.total(); got != 3*time.Second {
		t.Errorf("backoff total: got %v, want 3s (1s + 2s)", got)
	}
}

func TestFetchApplicationsPage_GivesUpAfterMaxRetries(t *testing.T) {
	t.Parallel()
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv.URL, &fakeClock{})
	_, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	// 1 initial + 3 retries = 4 attempts.
	if calls != 4 {
		t.Errorf("calls: got %d, want 4 (1 + MaxRetries)", calls)
	}
}

func TestFetchApplicationsPage_4xxIsPermanentAndNotRetried(t *testing.T) {
	t.Parallel()
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv.URL, &fakeClock{})
	_, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1)
	if err == nil {
		t.Fatal("expected error for 400")
	}
	var rl *RateLimitError
	if errors.As(err, &rl) {
		t.Error("400 must not be a RateLimitError")
	}
	if calls != 1 {
		t.Errorf("4xx should not be retried: got %d calls, want 1", calls)
	}
}

func TestFetchApplicationsPage_AppliesThrottleDelayBeforeRequest(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"records":[]}`))
	}))
	t.Cleanup(srv.Close)

	clock := &fakeClock{}
	c, err := NewClient(Options{
		BaseURL:    srv.URL,
		Throttle:   ThrottleOptions{DelayBetweenRequests: 2 * time.Second},
		Retry:      RetryOptions{MaxRetries: 0},
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		Sleep:      clock.sleep,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if _, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1); err != nil {
		t.Fatalf("FetchApplicationsPage: %v", err)
	}
	if got := clock.total(); got != 2*time.Second {
		t.Errorf("throttle delay: got %v, want 2s", got)
	}
}

func TestNewClient_RejectsNonHTTPSAndEmptyBaseURL(t *testing.T) {
	t.Parallel()
	if _, err := NewClient(Options{BaseURL: ""}); err == nil {
		t.Error("empty base URL should error")
	}
	if _, err := NewClient(Options{BaseURL: "http://planit.example.com"}); err == nil {
		t.Error("non-HTTPS, non-localhost base URL should error")
	}
	// Localhost http is permitted for tests.
	if _, err := NewClient(Options{BaseURL: "http://127.0.0.1:8080"}); err != nil {
		t.Errorf("localhost http should be permitted: %v", err)
	}
}
