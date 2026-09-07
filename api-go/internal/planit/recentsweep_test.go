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

// TestBuildRecentSweepPath pins Lane E's looping recent-window start_date sweep
// query shape (GH#1134, ADR 0047): both a start_date and end_date bound,
// sort=-start_date (start_date is the sort field, so it MUST appear in select or
// PlanIt 400s), pg_sz=300, compress=on, a LIGHT projection
// (uid, area_id, app_state, decided_date, start_date — NOT the full ingest set,
// and deliberately WITHOUT last_different), and no auth param.
func TestBuildRecentSweepPath(t *testing.T) {
	t.Parallel()
	windowStart := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

	path := buildRecentSweepPath(windowStart, windowEnd, 300)
	u, err := url.Parse(path)
	if err != nil {
		t.Fatalf("parse built path %q: %v", path, err)
	}
	got := u.Query()

	if got.Get("start_date") != "2026-06-09" {
		t.Errorf("start_date: got %q, want 2026-06-09", got.Get("start_date"))
	}
	if got.Get("end_date") != "2026-09-07" {
		t.Errorf("end_date: got %q, want 2026-09-07", got.Get("end_date"))
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
		t.Error("recent-sweep query must not carry an auth param (national, not per-authority)")
	}

	fields := strings.Split(got.Get("select"), ",")
	if !containsString(fields, "start_date") {
		t.Errorf("select must contain the sort field start_date (PlanIt 400s otherwise): got %v", fields)
	}
	for _, want := range []string{"uid", "area_id", "app_state", "decided_date"} {
		if !containsString(fields, want) {
			t.Errorf("select must contain %q: got %v", want, fields)
		}
	}
	if containsString(fields, "last_different") {
		t.Errorf("select must NOT contain last_different (a re-index bumps it; nothing in Lane E's divergence test reads it): got %v", fields)
	}
	if containsString(fields, "other_fields") || containsString(fields, "reference") {
		t.Errorf("select must be the LIGHT projection, not the full ingest set: got %v", fields)
	}
}

// TestFetchRecentSweepPage_SendsExpectedQueryAndParsesResponse drives the client
// end to end against an httptest server on localhost (never planit.org.uk),
// mirroring the Lane D backfill test.
func TestFetchRecentSweepPage_SendsExpectedQueryAndParsesResponse(t *testing.T) {
	t.Parallel()
	body := `{"total":9824,"pg_sz":300,"from":0,"records":[
		{"uid":"26/0001/FUL","area_id":300,"app_state":"Undecided","start_date":"2026-08-30"},
		{"uid":"26/0002/FUL","area_id":301,"app_state":"Permitted","decided_date":"2026-09-01","start_date":"2026-08-28"}
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

	windowStart := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	res, err := c.FetchRecentSweepPage(context.Background(), windowStart, windowEnd, 0)
	if err != nil {
		t.Fatalf("FetchRecentSweepPage: %v", err)
	}

	if gotQuery.Get("start_date") != "2026-08-23" ||
		gotQuery.Get("end_date") != "2026-09-07" ||
		gotQuery.Get("sort") != "-start_date" ||
		gotQuery.Get("pg_sz") != "300" ||
		gotQuery.Get("compress") != "on" ||
		gotQuery.Has("auth") {
		t.Errorf("unexpected request query: %s", gotQuery.Encode())
	}

	if res.Total == nil || *res.Total != 9824 {
		t.Errorf("Total: got %v, want 9824", res.Total)
	}
	if len(res.Applications) != 2 {
		t.Fatalf("Applications: got %d, want 2 (%+v)", len(res.Applications), res.Applications)
	}
	if res.Applications[0].UID != "26/0001/FUL" || res.Applications[0].AreaID != 300 {
		t.Errorf("Applications[0]: got %+v", res.Applications[0])
	}
	if res.Applications[1].DecidedDate == nil {
		t.Errorf("Applications[1].DecidedDate: got nil, want parsed 2026-09-01")
	}
}

// TestFetchRecentSweepPage_RateLimited proves a 429 surfaces as *RateLimitError,
// identically to every other fetch method.
func TestFetchRecentSweepPage_RateLimited(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "45")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	clock := &fakeClock{}
	c := newTestClient(t, srv.URL, clock)

	_, err := c.FetchRecentSweepPage(context.Background(), time.Now(), time.Now(), 0)
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
	if rl.RetryAfter == nil || *rl.RetryAfter != 45*time.Second {
		t.Errorf("RetryAfter: got %v", rl.RetryAfter)
	}
}
