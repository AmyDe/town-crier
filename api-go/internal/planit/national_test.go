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

// TestBuildNationalDeltaPath pins the Lane A/B query shape (ADR 0041 / GH#962):
// the mask, the coarse different_start prefilter, the descending sort, a
// select projection containing the sort field, no auth param, pg_sz=300, and
// compress=on.
func TestBuildNationalDeltaPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		mask          MaskParam
		wantMaskParam string
	}{
		{name: "lane A masks on start_date", mask: MaskStartDate, wantMaskParam: "start_date"},
		{name: "lane B masks on decided_start", mask: MaskDecidedStart, wantMaskParam: "decided_start"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q := NationalDeltaQuery{
				DifferentStart: time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
				Mask:           tc.mask,
				MaskCutoff:     time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
				StartIndex:     300,
			}
			path := buildNationalDeltaPath(q)

			u, err := url.Parse(path)
			if err != nil {
				t.Fatalf("parse built path %q: %v", path, err)
			}
			got := u.Query()

			if got.Get("different_start") != "2026-07-14" {
				t.Errorf("different_start prefilter: got %q, want 2026-07-14", got.Get("different_start"))
			}
			if got.Get(tc.wantMaskParam) != "2026-04-15" {
				t.Errorf("mask %s: got %q, want 2026-04-15", tc.wantMaskParam, got.Get(tc.wantMaskParam))
			}
			if got.Get("sort") != "-last_different" {
				t.Errorf("sort: got %q, want -last_different", got.Get("sort"))
			}
			if got.Get("pg_sz") != "300" {
				t.Errorf("pg_sz: got %q, want 300", got.Get("pg_sz"))
			}
			if got.Get("compress") != "on" {
				t.Errorf("compress: got %q, want on", got.Get("compress"))
			}
			if got.Get("index") != "300" {
				t.Errorf("index: got %q, want 300", got.Get("index"))
			}
			if got.Has("auth") {
				t.Error("national query must not carry an auth param")
			}
			fields := strings.Split(got.Get("select"), ",")
			if !containsString(fields, "last_different") {
				t.Errorf("select must contain the sort field last_different: got %v", fields)
			}
		})
	}
}

// TestBuildReconciliationPath pins Lane C's light per-authority projection
// shape: scoped by auth, a different_start date bound (tc-tuge8/GH#971: PlanIt
// 400s "Spatial, date or search restrictions required in query" on a query
// with no date param at all -- confirmed from prod's
// reconciliation.sample_error_body span attribute), the light select set
// (containing the sort field), pg_sz=300, compress=on. differentStart is
// injected (not real time) so the assertion is deterministic.
func TestBuildReconciliationPath(t *testing.T) {
	t.Parallel()
	differentStart := time.Date(2025, 7, 17, 0, 0, 0, 0, time.UTC)
	path := buildReconciliationPath(99, 0, differentStart)
	u, err := url.Parse(path)
	if err != nil {
		t.Fatalf("parse built path %q: %v", path, err)
	}
	got := u.Query()

	if got.Get("auth") != "99" {
		t.Errorf("auth: got %q, want 99", got.Get("auth"))
	}
	if got.Get("different_start") != "2025-07-17" {
		t.Errorf("different_start: got %q, want 2025-07-17", got.Get("different_start"))
	}
	if got.Get("pg_sz") != "300" {
		t.Errorf("pg_sz: got %q, want 300", got.Get("pg_sz"))
	}
	if got.Get("compress") != "on" {
		t.Errorf("compress: got %q, want on", got.Get("compress"))
	}
	fields := strings.Split(got.Get("select"), ",")
	if !containsString(fields, "last_different") {
		t.Errorf("select must contain the sort field last_different: got %v", fields)
	}
	if containsString(fields, "name") {
		t.Errorf("reconciliation select must stay a light projection, not the full ingest set: got %v", fields)
	}
}

// TestBuildUIDPath pins Lane C's single-record hydration shape: id_match, the
// full ingest select set, a minimal page size.
func TestBuildUIDPath(t *testing.T) {
	t.Parallel()
	path := buildUIDPath("24/0001/FUL")
	u, err := url.Parse(path)
	if err != nil {
		t.Fatalf("parse built path %q: %v", path, err)
	}
	got := u.Query()

	if got.Get("id_match") != "24/0001/FUL" {
		t.Errorf("id_match: got %q, want 24/0001/FUL", got.Get("id_match"))
	}
	if got.Get("pg_sz") != "1" {
		t.Errorf("pg_sz: got %q, want 1", got.Get("pg_sz"))
	}
	fields := strings.Split(got.Get("select"), ",")
	if !containsString(fields, "area_id") {
		t.Errorf("uid hydration must request the full ingest set (area_id needed for authority partitioning): got %v", fields)
	}
}

// TestFetchNationalDeltaPage_SendsExpectedQueryAndParsesResponse drives the
// client end-to-end against an httptest server on localhost (never
// planit.org.uk): the request the client actually sends carries every
// mandatory query param, and the response maps back into the domain snapshot.
func TestFetchNationalDeltaPage_SendsExpectedQueryAndParsesResponse(t *testing.T) {
	t.Parallel()
	body := `{"total":1717,"pg_sz":300,"from":0,"records":[
		{"name":"26/0001","uid":"26/0001/FUL","area_name":"Camden","area_id":300,"address":"1 High St","description":"A shed","app_type":"Full","app_state":"Undecided","start_date":"2026-07-01","location_x":-0.1,"location_y":51.5,"last_different":"2026-07-14T09:00:00.123456"}
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

	q := NationalDeltaQuery{
		DifferentStart: time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC),
		Mask:           MaskStartDate,
		MaskCutoff:     time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC),
		StartIndex:     0,
	}
	res, err := c.FetchNationalDeltaPage(context.Background(), q)
	if err != nil {
		t.Fatalf("FetchNationalDeltaPage: %v", err)
	}

	if gotQuery.Get("different_start") != "2026-07-14" ||
		gotQuery.Get("start_date") != "2026-04-16" ||
		gotQuery.Get("sort") != "-last_different" ||
		gotQuery.Get("pg_sz") != "300" ||
		gotQuery.Get("compress") != "on" ||
		gotQuery.Has("auth") {
		t.Errorf("unexpected request query: %s", gotQuery.Encode())
	}

	if res.Total == nil || *res.Total != 1717 {
		t.Errorf("Total: got %v, want 1717", res.Total)
	}
	if len(res.Applications) != 1 || res.Applications[0].UID != "26/0001/FUL" {
		t.Fatalf("Applications: got %+v", res.Applications)
	}
	wantLastDifferent := time.Date(2026, 7, 14, 9, 0, 0, 123456000, time.UTC)
	if !res.Applications[0].LastDifferent.Equal(wantLastDifferent) {
		t.Errorf("LastDifferent: got %v, want %v", res.Applications[0].LastDifferent, wantLastDifferent)
	}
}

// TestFetchNationalDeltaPage_RateLimited proves a 429 surfaces as
// *RateLimitError, identically to FetchApplicationsPage.
func TestFetchNationalDeltaPage_RateLimited(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	clock := &fakeClock{}
	c := newTestClient(t, srv.URL, clock)

	_, err := c.FetchNationalDeltaPage(context.Background(), NationalDeltaQuery{
		DifferentStart: time.Now(),
		Mask:           MaskStartDate,
		MaskCutoff:     time.Now(),
	})
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("expected *RateLimitError, got %T: %v", err, err)
	}
	if rl.RetryAfter == nil || *rl.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter: got %v", rl.RetryAfter)
	}
}

// TestFetchByUID_SendsExpectedQuery pins the single-uid hydration lookup.
func TestFetchByUID_SendsExpectedQuery(t *testing.T) {
	t.Parallel()
	body := `{"records":[{"name":"26/0001","uid":"26/0001/FUL","area_id":300,"last_different":"2026-07-14T09:00:00"}]}`
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	clock := &fakeClock{}
	c := newTestClient(t, srv.URL, clock)

	res, err := c.FetchByUID(context.Background(), "26/0001/FUL")
	if err != nil {
		t.Fatalf("FetchByUID: %v", err)
	}
	if gotQuery.Get("id_match") != "26/0001/FUL" || gotQuery.Get("pg_sz") != "1" {
		t.Errorf("unexpected request query: %s", gotQuery.Encode())
	}
	if len(res.Applications) != 1 || res.Applications[0].AreaID != 300 {
		t.Fatalf("Applications: got %+v", res.Applications)
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
