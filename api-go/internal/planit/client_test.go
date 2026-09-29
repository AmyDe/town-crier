package planit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchWindowPage_ParsesRecordsAndTotal(t *testing.T) {
	t.Parallel()
	body := `{"total":42,"pg_sz":100,"from":0,"records":[
		{"name":"24/0001","uid":"24/0001/FUL","area_name":"Test","area_id":99,"address":"1 High St","postcode":"AB1 2CD","description":"A shed","app_type":"Full","app_state":"Undecided","app_size":"Small","start_date":"2026-06-01","location_x":-0.1,"location_y":51.5,"url":"http://x","link":"http://y","last_different":"2026-06-10T09:00:00Z"}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("auth") != "99" || q.Get("index") != "0" || q.Get("sort") != "last_different" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	c := newWindowClient(t, srv.URL, func(o *Options) { o.AreaID = 99 })

	res, err := c.FetchWindowPage(context.Background(), testQuery(0))
	if err != nil {
		t.Fatalf("FetchWindowPage: %v", err)
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

// TestFetchWindowPage_TruncatedPageWithTotal_HasMorePagesTrue pins the
// truncation-safety acceptance criterion: a response truncated well under the
// nominal page size (e.g. by PlanIt's 1MB body cap) must still report
// HasMorePages=true whenever from+len(records) < total, and From must echo the
// response's own from (falling back to startIndex only when absent).
func TestFetchWindowPage_TruncatedPageWithTotal_HasMorePagesTrue(t *testing.T) {
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

	c := newWindowClient(t, srv.URL, nil)
	res, err := c.FetchWindowPage(context.Background(), testQuery(0))
	if err != nil {
		t.Fatalf("FetchWindowPage: %v", err)
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

// TestFetchWindowPage_MissingTotalFallsBackToLengthHeuristic covers a
// response that omits total entirely: HasMorePages must fall back to the
// page-size length heuristic (>= the 300 page size).
func TestFetchWindowPage_MissingTotalFallsBackToLengthHeuristic(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	sb.WriteString(`{"records":[`)
	for i := range 300 {
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

	c := newWindowClient(t, srv.URL, nil)
	res, err := c.FetchWindowPage(context.Background(), testQuery(0))
	if err != nil {
		t.Fatalf("FetchWindowPage: %v", err)
	}
	if res.Total != nil {
		t.Fatalf("Total: got %v, want nil (omitted by response)", res.Total)
	}
	if !res.HasMorePages {
		t.Error("HasMorePages should be true for a full (300-record) page when total is unknown")
	}
}

// TestFetchWindowPage_ZeroRecordsWithMoreRemaining_ReturnsError pins the
// zero-progress guard: PlanIt reporting total > from while returning zero
// records this fetch must error rather than let a resuming caller livelock at
// the same index forever.
func TestFetchWindowPage_ZeroRecordsWithMoreRemaining_ReturnsError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total":500,"from":200,"records":[]}`))
	}))
	t.Cleanup(srv.Close)

	c := newWindowClient(t, srv.URL, nil)
	_, err := c.FetchWindowPage(context.Background(), testQuery(200))
	if !errors.Is(err, ErrZeroProgress) {
		t.Fatalf("expected ErrZeroProgress, got %v", err)
	}
}

// TestFetchWindowPage_ZeroRecordsAtTotalIsNotAnError covers the normal
// natural end: zero records with from == total (nothing remains) must NOT
// trip the zero-progress guard.
func TestFetchWindowPage_ZeroRecordsAtTotalIsNotAnError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total":200,"from":200,"records":[]}`))
	}))
	t.Cleanup(srv.Close)

	c := newWindowClient(t, srv.URL, nil)
	res, err := c.FetchWindowPage(context.Background(), testQuery(200))
	if err != nil {
		t.Fatalf("FetchWindowPage: %v", err)
	}
	if res.HasMorePages {
		t.Error("HasMorePages should be false when from == total")
	}
}

func TestFetchWindowPage_429SurfacesRateLimitWithRetryAfterAndIsNotRetried(t *testing.T) {
	t.Parallel()
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	c := newWindowClient(t, srv.URL, nil)
	_, err := c.FetchWindowPage(context.Background(), testQuery(1))

	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected RateLimitError, got %v", err)
	}
	if rl.RetryAfter == nil || *rl.RetryAfter != 120*time.Second {
		t.Errorf("RetryAfter: got %v, want 120s", rl.RetryAfter)
	}
	// 429 must NOT be retried (the Pacer owns backoff).
	if calls != 1 {
		t.Errorf("429 should not be retried: got %d calls, want 1", calls)
	}
}

func TestFetchWindowPage_429WithoutHeaderHasNilRetryAfter(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	c := newWindowClient(t, srv.URL, nil)
	_, err := c.FetchWindowPage(context.Background(), testQuery(1))

	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected RateLimitError, got %v", err)
	}
	if rl.RetryAfter != nil {
		t.Errorf("RetryAfter should be nil when header absent, got %v", rl.RetryAfter)
	}
}

// TestClassifyCapturesErrorBody pins that PlanIt's 400 body, which carries the
// failure reason, is surfaced on the returned *HTTPError.
func TestClassifyCapturesErrorBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"42703: column \"last_different\" does not exist"}`))
	}))
	t.Cleanup(srv.Close)

	c := newWindowClient(t, srv.URL, nil)
	_, err := c.FetchWindowPage(context.Background(), testQuery(1))

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

// TestClassify429DoesNotCaptureBody pins that a 429 with a body still yields a
// *RateLimitError and never an *HTTPError.
func TestClassify429DoesNotCaptureBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	t.Cleanup(srv.Close)

	c := newWindowClient(t, srv.URL, nil)
	_, err := c.FetchWindowPage(context.Background(), testQuery(1))

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
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(longBody))
	}))
	t.Cleanup(srv.Close)

	c := newWindowClient(t, srv.URL, nil)
	_, err := c.FetchWindowPage(context.Background(), testQuery(1))

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

func testQuery(index int) WindowQuery {
	return WindowQuery{Work: WorkWindowStart, Axis: AxisStart, From: day(time.January, 1), To: day(time.January, 1), Index: index}
}
