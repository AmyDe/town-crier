package planit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	body := `{"total":42,"pg_sz":100,"from":0,"records":[
		{"name":"24/0001","uid":"24/0001/FUL","area_name":"Test","area_id":99,"address":"1 High St","postcode":"AB1 2CD","description":"A shed","app_type":"Full","app_state":"Undecided","app_size":"Small","start_date":"2026-06-01","location_x":-0.1,"location_y":51.5,"url":"http://x","link":"http://y","last_different":"2026-06-10T09:00:00Z"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the query parameters: pg_sz, sort, index, auth, different_start.
		q := r.URL.Query()
		if q.Get("auth") != "99" || q.Get("index") != "0" || q.Get("sort") != "last_different" || q.Get("different_start") != "2026-06-09" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	clock := &fakeClock{}
	c := newTestClient(t, srv.URL, clock)

	ds := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	res, err := c.FetchApplicationsPage(context.Background(), 99, &ds, 0, false)
	if err != nil {
		t.Fatalf("FetchApplicationsPage: %v", err)
	}
	if res.Total == nil || *res.Total != 42 {
		t.Errorf("total: got %v, want 42", res.Total)
	}
	if res.From != 0 {
		t.Errorf("From: got %d, want 0", res.From)
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
	// from(0) + 1 record < total(42): more records remain.
	if !res.HasMorePages {
		t.Error("HasMorePages should be true when from+len(records) < total")
	}
}

// TestFetchApplicationsPage_TruncatedPageWithTotal_HasMorePagesTrue pins the
// truncation-safety acceptance criterion: a response truncated well under the
// nominal page size (e.g. by PlanIt's 1MB body cap) must still report
// HasMorePages=true whenever from+len(records) < total, and From must echo the
// response's own from (falling back to startIndex only when absent).
func TestFetchApplicationsPage_TruncatedPageWithTotal_HasMorePagesTrue(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	sb.WriteString(`{"total":500,"from":0,"records":[`)
	for i := range 87 {
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
	res, err := c.FetchApplicationsPage(context.Background(), 1, nil, 0, false)
	if err != nil {
		t.Fatalf("FetchApplicationsPage: %v", err)
	}
	if !res.HasMorePages {
		t.Error("truncated page (87 < total 500) must still report HasMorePages=true")
	}
	if res.From != 0 {
		t.Errorf("From: got %d, want 0", res.From)
	}
	if len(res.Applications) != 87 {
		t.Errorf("applications: got %d, want 87 (truncated)", len(res.Applications))
	}
}

// TestFetchApplicationsPage_MissingTotalFallsBackToLengthHeuristic covers a
// response that omits total entirely: HasMorePages must fall back to the
// page-size length heuristic (>= configured page size).
func TestFetchApplicationsPage_MissingTotalFallsBackToLengthHeuristic(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	sb.WriteString(`{"records":[`)
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
	res, err := c.FetchApplicationsPage(context.Background(), 1, nil, 0, false)
	if err != nil {
		t.Fatalf("FetchApplicationsPage: %v", err)
	}
	if res.Total != nil {
		t.Fatalf("Total: got %v, want nil (omitted by response)", res.Total)
	}
	if !res.HasMorePages {
		t.Error("HasMorePages should be true for a full (100-record) page when total is unknown")
	}
}

// TestFetchApplicationsPage_ZeroRecordsWithMoreRemaining_ReturnsError pins the
// zero-progress guard: PlanIt reporting total > from while returning zero
// records this fetch must error rather than let a resuming caller livelock at
// the same index forever.
func TestFetchApplicationsPage_ZeroRecordsWithMoreRemaining_ReturnsError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total":500,"from":200,"records":[]}`))
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv.URL, &fakeClock{})
	_, err := c.FetchApplicationsPage(context.Background(), 1, nil, 200, false)
	if !errors.Is(err, ErrZeroProgress) {
		t.Fatalf("expected ErrZeroProgress, got %v", err)
	}
}

// TestFetchApplicationsPage_ZeroRecordsAtTotalIsNotAnError covers the normal
// natural end: zero records with from == total (nothing remains) must NOT
// trip the zero-progress guard.
func TestFetchApplicationsPage_ZeroRecordsAtTotalIsNotAnError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total":200,"from":200,"records":[]}`))
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv.URL, &fakeClock{})
	res, err := c.FetchApplicationsPage(context.Background(), 1, nil, 200, false)
	if err != nil {
		t.Fatalf("FetchApplicationsPage: %v", err)
	}
	if res.HasMorePages {
		t.Error("HasMorePages should be false when from == total")
	}
}

func TestBuildPath_IndexAndSortDirections(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		startIndex int
		descending bool
		pageSize   int
		wantIndex  string
		wantSort   string
		wantPgSz   string
	}{
		{"ascending default", 0, false, 100, "0", "last_different", "100"},
		{"descending probe", 0, true, 100, "0", "-last_different", "100"},
		{"non-zero resume index", 1700, false, 300, "1700", "last_different", "300"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := buildPath(99, nil, tc.startIndex, tc.descending, tc.pageSize)
			u, err := url.Parse("http://x" + path)
			if err != nil {
				t.Fatalf("parse built path: %v", err)
			}
			q := u.Query()
			if got := q.Get("index"); got != tc.wantIndex {
				t.Errorf("index: got %q, want %q (path=%s)", got, tc.wantIndex, path)
			}
			if got := q.Get("sort"); got != tc.wantSort {
				t.Errorf("sort: got %q, want %q (path=%s)", got, tc.wantSort, path)
			}
			if got := q.Get("pg_sz"); got != tc.wantPgSz {
				t.Errorf("pg_sz: got %q, want %q (path=%s)", got, tc.wantPgSz, path)
			}
			if got := q.Get("auth"); got != "99" {
				t.Errorf("auth: got %q, want 99 (path=%s)", got, path)
			}
			if strings.Contains(path, "page=") {
				t.Errorf("path must not contain the legacy page= parameter: %s", path)
			}
		})
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
	_, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1, false)

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
	_, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1, false)

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
	res, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1, false)
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
	_, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1, false)
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
	_, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1, false)
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

// TestClassifyCapturesErrorBody pins tc-tuge8/GH#971 root cause 1: PlanIt's
// 400 body carries the actual failure reason (e.g. a bad column name), and it
// was previously discarded entirely. A non-2xx, non-429 response must surface
// its body on the returned *HTTPError.
func TestClassifyCapturesErrorBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"42703: column \"last_different\" does not exist"}`))
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv.URL, &fakeClock{})
	_, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1, false)

	var herr *HTTPError
	if !errors.As(err, &herr) {
		t.Fatalf("expected *HTTPError, got %v", err)
	}
	if herr.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode: got %d, want 400", herr.StatusCode)
	}
	wantBody := `{"error":"42703: column \"last_different\" does not exist"}`
	if herr.Body != wantBody {
		t.Errorf("Body: got %q, want %q", herr.Body, wantBody)
	}
	if !strings.Contains(herr.Error(), "42703") {
		t.Errorf("Error() should include the captured body, got %q", herr.Error())
	}
}

// TestClassify429DoesNotCaptureBody pins the "429 branch stays completely
// untouched" requirement: even when a 429 response carries a body, classify
// must still yield a *RateLimitError (never an *HTTPError), and the body must
// never be read from the 429 path.
func TestClassify429DoesNotCaptureBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv.URL, &fakeClock{})
	_, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1, false)

	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected *RateLimitError, got %v", err)
	}
	if rl.RetryAfter == nil || *rl.RetryAfter != 60*time.Second {
		t.Errorf("RetryAfter: got %v, want 60s", rl.RetryAfter)
	}
	var herr *HTTPError
	if errors.As(err, &herr) {
		t.Error("a 429 must never become an *HTTPError")
	}
}

// TestClassifyTruncatesLongErrorBody pins the 512-byte bound on a captured
// error body, so a hostile or broken upstream cannot balloon the eventual
// OTel span attribute.
func TestClassifyTruncatesLongErrorBody(t *testing.T) {
	t.Parallel()
	longBody := strings.Repeat("x", 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// 500 is not retryable by isRetryable (only 502/503/504/408 are), so
		// this reaches classify on the first and only attempt.
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(longBody))
	}))
	t.Cleanup(srv.Close)

	c := newTestClient(t, srv.URL, &fakeClock{})
	_, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1, false)

	var herr *HTTPError
	if !errors.As(err, &herr) {
		t.Fatalf("expected *HTTPError, got %v", err)
	}
	if len(herr.Body) != 512 {
		t.Errorf("Body length: got %d, want 512 (truncated)", len(herr.Body))
	}
	if herr.Body != longBody[:512] {
		t.Error("Body should be exactly the first 512 bytes of the response")
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
	if _, err := c.FetchApplicationsPage(context.Background(), 1, nil, 1, false); err != nil {
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
