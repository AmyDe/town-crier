package planit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakePlanItMetrics records the http-error calls the client makes. It satisfies
// the planit package's consumer-side httpErrorRecorder interface.
type fakePlanItMetrics struct {
	statuses    []int
	authorities []int
}

func (f *fakePlanItMetrics) PlanItHTTPError(_ context.Context, statusCode, authorityID int) {
	f.statuses = append(f.statuses, statusCode)
	f.authorities = append(f.authorities, authorityID)
}

func TestFetchApplicationsPage_RecordsHTTPErrorOnPermanent4xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	rec := &fakePlanItMetrics{}
	clock := &fakeClock{}
	c, err := NewClient(Options{
		BaseURL:    srv.URL,
		Throttle:   ThrottleOptions{DelayBetweenRequests: 0},
		Retry:      RetryOptions{MaxRetries: 0, InitialBackoff: time.Second, RateLimitBackoff: 5 * time.Second},
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		Sleep:      clock.sleep,
		Metrics:    rec,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := c.FetchApplicationsPage(context.Background(), 99, nil, 1); err == nil {
		t.Fatal("expected an error for a 404")
	}
	if len(rec.statuses) != 1 || rec.statuses[0] != http.StatusNotFound {
		t.Errorf("PlanItHTTPError statuses = %v, want [404]", rec.statuses)
	}
	if len(rec.authorities) != 1 || rec.authorities[0] != 99 {
		t.Errorf("PlanItHTTPError authorities = %v, want [99]", rec.authorities)
	}
}

func TestFetchApplicationsPage_DoesNotRecordHTTPErrorOnSuccess(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":0,"records":[]}`))
	}))
	t.Cleanup(srv.Close)

	rec := &fakePlanItMetrics{}
	clock := &fakeClock{}
	c, err := NewClient(Options{
		BaseURL:    srv.URL,
		Throttle:   ThrottleOptions{DelayBetweenRequests: 0},
		Retry:      RetryOptions{MaxRetries: 0},
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		Sleep:      clock.sleep,
		Metrics:    rec,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if _, err := c.FetchApplicationsPage(context.Background(), 99, nil, 1); err != nil {
		t.Fatalf("FetchApplicationsPage: %v", err)
	}
	if len(rec.statuses) != 0 {
		t.Errorf("PlanItHTTPError must not fire on success: %v", rec.statuses)
	}
}
