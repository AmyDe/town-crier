package planit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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

func TestFetchWindowPage_RecordsHTTPErrorOnPermanent4xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	rec := &fakePlanItMetrics{}
	c := newWindowClient(t, srv.URL, func(o *Options) {
		o.AreaID = 99
		o.Metrics = rec
	})

	if _, err := c.FetchWindowPage(context.Background(), testQuery(1)); err == nil {
		t.Fatal("expected an error for a 404")
	}
	if len(rec.statuses) != 1 || rec.statuses[0] != http.StatusNotFound {
		t.Errorf("PlanItHTTPError statuses = %v, want [404]", rec.statuses)
	}
	if len(rec.authorities) != 1 || rec.authorities[0] != 99 {
		t.Errorf("PlanItHTTPError authorities = %v, want [99]", rec.authorities)
	}
}

func TestFetchWindowPage_DoesNotRecordHTTPErrorOnSuccess(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":0,"records":[]}`))
	}))
	t.Cleanup(srv.Close)

	rec := &fakePlanItMetrics{}
	c := newWindowClient(t, srv.URL, func(o *Options) {
		o.AreaID = 99
		o.Metrics = rec
	})

	if _, err := c.FetchWindowPage(context.Background(), testQuery(1)); err != nil {
		t.Fatalf("FetchWindowPage: %v", err)
	}
	if len(rec.statuses) != 0 {
		t.Errorf("PlanItHTTPError must not fire on success: %v", rec.statuses)
	}
}
