package planit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestBuildBackfillPath pins Lane D's national, date-windowed backward-sweep
// query shape (GH#967): both a start_date and end_date bound (never an
// unbounded one-sided window — the shape ADR 0041 explicitly measured at
// 11.7s, not the 45s/total:null shape it ruled out), sort=-start_date, the
// full ingest select projection (so the sweep can enrich every GH#935
// field), pg_sz=300, compress=on, and critically NO auth param — this is a
// national sweep, not a per-authority one.
func TestBuildBackfillPath(t *testing.T) {
	t.Parallel()
	windowStart := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)

	path := buildBackfillPath(windowStart, windowEnd, 300)
	u, err := url.Parse(path)
	if err != nil {
		t.Fatalf("parse built path %q: %v", path, err)
	}
	got := u.Query()

	if got.Get("start_date") != "2026-04-16" {
		t.Errorf("start_date: got %q, want 2026-04-16", got.Get("start_date"))
	}
	if got.Get("end_date") != "2026-07-15" {
		t.Errorf("end_date: got %q, want 2026-07-15", got.Get("end_date"))
	}
	if got.Get("sort") != "-start_date" {
		t.Errorf("sort: got %q, want -start_date", got.Get("sort"))
	}
	if got.Get("pg_sz") != "300" {
		t.Errorf("pg_sz: got %q, want 300", got.Get("pg_sz"))
	}
	if got.Get("index") != "300" {
		t.Errorf("index: got %q, want 300", got.Get("index"))
	}
	if got.Get("compress") != "on" {
		t.Errorf("compress: got %q, want on", got.Get("compress"))
	}
	if got.Has("auth") {
		t.Error("backfill query must not carry an auth param (national, not per-authority)")
	}
	fields := strings.Split(got.Get("select"), ",")
	if !containsString(fields, "start_date") {
		t.Errorf("select must contain the sort field start_date: got %v", fields)
	}
	if !containsString(fields, "reference") || !containsString(fields, "last_scraped") {
		t.Errorf("select must be the full ingest projection (GH#935 fields included): got %v", fields)
	}
}

// TestFetchBackfillPage_SendsExpectedQueryAndParsesResponse drives the client
// end-to-end against an httptest server on localhost (never planit.org.uk).
func TestFetchBackfillPage_SendsExpectedQueryAndParsesResponse(t *testing.T) {
	t.Parallel()
	body := `{"total":10,"pg_sz":300,"from":0,"records":[
		{"name":"19/0001","uid":"19/0001/FUL","area_name":"Camden","area_id":300,"address":"1 High St","description":"A shed","app_type":"Full","app_state":"Permitted","start_date":"2019-04-20","location_x":-0.1,"location_y":51.5,"last_different":"2026-07-14T09:00:00.000000"}
	]}`
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	clock := &fakeClock{}
	c := newTestClient(t, srv.URL, clock)

	windowStart := time.Date(2019, 1, 20, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2019, 4, 20, 0, 0, 0, 0, time.UTC)
	res, err := c.FetchBackfillPage(context.Background(), windowStart, windowEnd, 0)
	if err != nil {
		t.Fatalf("FetchBackfillPage: %v", err)
	}

	if gotQuery.Get("start_date") != "2019-01-20" ||
		gotQuery.Get("end_date") != "2019-04-20" ||
		gotQuery.Get("sort") != "-start_date" ||
		gotQuery.Get("pg_sz") != "300" ||
		gotQuery.Get("compress") != "on" ||
		gotQuery.Has("auth") {
		t.Errorf("unexpected request query: %s", gotQuery.Encode())
	}

	if res.Total == nil || *res.Total != 10 {
		t.Errorf("Total: got %v, want 10", res.Total)
	}
	if len(res.Applications) != 1 || res.Applications[0].UID != "19/0001/FUL" {
		t.Fatalf("Applications: got %+v", res.Applications)
	}
}

// TestFetchBackfillPage_RateLimited proves a 429 surfaces as *RateLimitError,
// identically to every other fetch method.
func TestFetchBackfillPage_RateLimited(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	clock := &fakeClock{}
	c := newTestClient(t, srv.URL, clock)

	_, err := c.FetchBackfillPage(context.Background(), time.Now(), time.Now(), 0)
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
	if rl.RetryAfter == nil || *rl.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter: got %v", rl.RetryAfter)
	}
}
