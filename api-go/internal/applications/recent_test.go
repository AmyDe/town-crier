package applications

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

// fakeRecentStore is a hand-written double for the recent-by-authority read. It
// honours cap the way Cosmos TOP @cap does (so the handler's cap/limit logic is
// exercised against realistic bounded results) and serves a whole-partition
// status breakdown independently of the bounded read, so a test can prove the
// rendered Total is the sum of those buckets — not the bounded read length nor
// the rendered slice.
type fakeRecentStore struct {
	apps         []PlanningApplication
	err          error
	breakdown    []StateCount
	breakdownErr error

	lastAuthorityCode          string
	lastCap                    int
	breakdownCalled            bool
	lastBreakdownAuthorityCode string
}

func (f *fakeRecentStore) RecentByAuthority(_ context.Context, authorityCode string, cap int) ([]PlanningApplication, error) {
	f.lastAuthorityCode = authorityCode
	f.lastCap = cap
	if f.err != nil {
		return nil, f.err
	}
	if cap >= 0 && cap < len(f.apps) {
		return f.apps[:cap], nil
	}
	return f.apps, nil
}

func (f *fakeRecentStore) BreakdownByAuthority(_ context.Context, authorityCode string) ([]StateCount, error) {
	f.breakdownCalled = true
	f.lastBreakdownAuthorityCode = authorityCode
	if f.breakdownErr != nil {
		return nil, f.breakdownErr
	}
	return f.breakdown, nil
}

// breakdownOf builds a []StateCount from raw appState/count pairs, for seeding the
// fake's whole-partition breakdown.
func breakdownOf(pairs ...StateCount) []StateCount {
	out := make([]StateCount, 0, len(pairs))
	out = append(out, pairs...)
	return out
}

// recentApps builds n distinct planning applications for the handler tests.
func recentApps(t *testing.T, n int) []PlanningApplication {
	t.Helper()
	base := testApplication(t)
	apps := make([]PlanningApplication, 0, n)
	for i := range n {
		a := base
		a.Name = "24/" + strconv.Itoa(i) + "/FUL"
		apps = append(apps, a)
	}
	return apps
}

// appsByState builds permitted+rejected distinct applications with the named raw
// appStates, for asserting the status breakdown spans the bounded read.
func appsByState(t *testing.T, permitted, rejected int) []PlanningApplication {
	t.Helper()
	base := testApplication(t)
	apps := make([]PlanningApplication, 0, permitted+rejected)
	i := 0
	add := func(state string, n int) {
		for range n {
			a := base
			a.Name = "24/" + strconv.Itoa(i) + "/FUL"
			st := state
			a.AppState = &st
			apps = append(apps, a)
			i++
		}
	}
	add("Permitted", permitted)
	add("Rejected", rejected)
	return apps
}

func serveRecent(t *testing.T, store recentStore, buildKey, providedKey, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	RecentRoutes(mux, store, buildKey, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	if providedKey != "" {
		req.Header.Set("X-Build-Key", providedKey)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// recentBody decodes the response into a loosely-typed map for shape assertions.
func recentBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return got
}

func TestRecentHandler_Returns200WithValidKeyAndNonNullArray(t *testing.T) {
	t.Parallel()
	// The whole-partition breakdown sums to 1234, deliberately distinct from
	// len(apps) (2), to prove total is the sum of the breakdown buckets, not the
	// bounded read length.
	store := &fakeRecentStore{
		apps: recentApps(t, 2),
		breakdown: breakdownOf(
			StateCount{AppState: strPtr("Permitted"), Count: 1000},
			StateCount{AppState: strPtr("Rejected"), Count: 234},
		),
	}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}
	// The handler reads the bounded cap (200) and computes the whole-partition
	// breakdown, both scoped to the numeric id -> code.
	if store.lastAuthorityCode != "471" || store.lastCap != 200 {
		t.Errorf("store read call: authorityCode=%q cap=%d, want \"471\" 200", store.lastAuthorityCode, store.lastCap)
	}
	if !store.breakdownCalled || store.lastBreakdownAuthorityCode != "471" {
		t.Errorf("breakdown call: called=%v authorityCode=%q, want true \"471\"", store.breakdownCalled, store.lastBreakdownAuthorityCode)
	}
	got := recentBody(t, rec)
	if got["authorityId"].(float64) != 471 {
		t.Errorf("authorityId: got %v, want 471", got["authorityId"])
	}
	if got["areaName"] != "City of London" {
		t.Errorf("areaName: got %v, want City of London", got["areaName"])
	}
	apps, ok := got["applications"].([]any)
	if !ok {
		t.Fatalf("applications must be a non-null array, got %T (%v)", got["applications"], got["applications"])
	}
	if len(apps) != 2 {
		t.Errorf("applications length: got %d, want 2", len(apps))
	}
	if got["total"].(float64) != 1234 {
		t.Errorf("total: got %v, want 1234 (sum of the breakdown buckets)", got["total"])
	}
	if _, present := got["totalCapped"]; present {
		t.Errorf("totalCapped must be gone from the wire shape, got %v", got["totalCapped"])
	}
	if _, present := got["statusBreakdown"]; !present {
		t.Errorf("statusBreakdown must be present: %v", got)
	}
	// The wire shape of an application carries the SEO-relevant fields, now
	// including lastDifferent (the list's DESC sort key, for the "Last updated" card).
	first := apps[0].(map[string]any)
	for _, key := range []string{"uid", "name", "address", "description", "appState", "startDate", "lastDifferent", "link", "url"} {
		if _, present := first[key]; !present {
			t.Errorf("application missing field %q: %v", key, first)
		}
	}
	if first["lastDifferent"] != "2026-03-02T09:30:00+00:00" {
		t.Errorf("lastDifferent: got %v, want 2026-03-02T09:30:00+00:00", first["lastDifferent"])
	}
}

func TestRecentHandler_RejectsMissingKey(t *testing.T) {
	t.Parallel()
	store := &fakeRecentStore{apps: recentApps(t, 2)}

	rec := serveRecent(t, store, "buildkey", "", "/v1/authorities/471/applications")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("401 must be bodyless (backfilled downstream), got %s", rec.Body)
	}
	if store.lastCap != 0 || store.breakdownCalled {
		t.Errorf("store must not be hit when the key is missing")
	}
}

func TestRecentHandler_EmptyConfiguredKeyRejectsAll(t *testing.T) {
	t.Parallel()
	store := &fakeRecentStore{apps: recentApps(t, 2)}

	rec := serveRecent(t, store, "", "anything", "/v1/authorities/471/applications")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("empty configured key must reject all: got %d, want 401", rec.Code)
	}
	if store.lastCap != 0 || store.breakdownCalled {
		t.Errorf("store must not be hit when the configured key is empty")
	}
}

func TestRecentHandler_LimitsReturnedApplications(t *testing.T) {
	t.Parallel()
	// Breakdown sums to 73, distinct from len(apps) (50) and the limit (10).
	store := &fakeRecentStore{
		apps: recentApps(t, 50),
		breakdown: breakdownOf(
			StateCount{AppState: strPtr("Permitted"), Count: 73},
		),
	}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications?limit=10")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	got := recentBody(t, rec)
	apps := got["applications"].([]any)
	if len(apps) != 10 {
		t.Errorf("applications length: got %d, want 10 (the limit)", len(apps))
	}
	// total is the sum of the whole-partition breakdown, not the rendered slice nor
	// the bounded read.
	if got["total"].(float64) != 73 {
		t.Errorf("total: got %v, want 73 (sum of breakdown buckets)", got["total"])
	}
}

func TestRecentHandler_DefaultLimitIs30(t *testing.T) {
	t.Parallel()
	store := &fakeRecentStore{
		apps:      recentApps(t, 50),
		breakdown: breakdownOf(StateCount{AppState: strPtr("Permitted"), Count: 50}),
	}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications")

	got := recentBody(t, rec)
	apps := got["applications"].([]any)
	if len(apps) != 30 {
		t.Errorf("applications length: got %d, want 30 (default limit)", len(apps))
	}
}

func TestRecentHandler_LimitHardCappedAt100(t *testing.T) {
	t.Parallel()
	store := &fakeRecentStore{
		apps:      recentApps(t, 150),
		breakdown: breakdownOf(StateCount{AppState: strPtr("Permitted"), Count: 150}),
	}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications?limit=999")

	got := recentBody(t, rec)
	apps := got["applications"].([]any)
	if len(apps) != 100 {
		t.Errorf("applications length: got %d, want 100 (hard max limit)", len(apps))
	}
}

func TestRecentHandler_ExactTotalIndependentOfSaturatedRead(t *testing.T) {
	t.Parallel()
	// The bounded read saturates at cap (200) but the whole-partition breakdown
	// sums to far more: total must be that sum, and the render slice clamps to the
	// limit, not the total (the bug the bounded-read clamp guards against).
	store := &fakeRecentStore{
		apps: recentApps(t, 200),
		breakdown: breakdownOf(
			StateCount{AppState: strPtr("Permitted"), Count: 6000},
			StateCount{AppState: strPtr("Rejected"), Count: 2421},
		),
	}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications")

	got := recentBody(t, rec)
	if got["total"].(float64) != 8421 {
		t.Errorf("total: got %v, want 8421 (sum of breakdown buckets)", got["total"])
	}
	if apps := got["applications"].([]any); len(apps) != 30 {
		t.Errorf("applications length: got %d, want 30 (default limit, NOT the total)", len(apps))
	}
}

func TestRecentHandler_StatusBreakdownIsWholePartitionEchoedVerbatim(t *testing.T) {
	t.Parallel()
	// The store's whole-partition breakdown sums to 3771 — far beyond the bounded
	// read (40 cards) and the rendered slice (30). The handler must echo the
	// store's breakdown verbatim, order preserved, and set total to its sum.
	store := &fakeRecentStore{
		apps: appsByState(t, 25, 15),
		breakdown: breakdownOf(
			StateCount{AppState: strPtr("Permitted"), Count: 2100},
			StateCount{AppState: strPtr("Rejected"), Count: 900},
			StateCount{AppState: strPtr("Conditions"), Count: 771},
		),
	}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications")

	got := recentBody(t, rec)
	breakdown, ok := got["statusBreakdown"].([]any)
	if !ok {
		t.Fatalf("statusBreakdown must be an array, got %T", got["statusBreakdown"])
	}
	if len(breakdown) != 3 {
		t.Fatalf("statusBreakdown length: got %d, want 3 (%v)", len(breakdown), breakdown)
	}
	// Echoed verbatim from the store, order preserved.
	wantStates := []string{"Permitted", "Rejected", "Conditions"}
	wantCounts := []float64{2100, 900, 771}
	for i, row := range breakdown {
		m := row.(map[string]any)
		if m["appState"] != wantStates[i] || m["count"].(float64) != wantCounts[i] {
			t.Errorf("breakdown[%d]: got %v, want %s/%v", i, m, wantStates[i], wantCounts[i])
		}
	}
	// total is the sum of the breakdown buckets (2100+900+771), equal to the
	// rendered "tracking N applications" lead line.
	if got["total"].(float64) != 3771 {
		t.Errorf("total: got %v, want 3771 (sum of breakdown buckets)", got["total"])
	}
}

func TestRecentHandler_EmptyResultIsNonNullArray(t *testing.T) {
	t.Parallel()
	store := &fakeRecentStore{apps: nil, breakdown: nil}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	got := recentBody(t, rec)
	apps, ok := got["applications"].([]any)
	if !ok {
		t.Fatalf("applications must be a non-null array even when empty, got %T", got["applications"])
	}
	if len(apps) != 0 {
		t.Errorf("applications length: got %d, want 0", len(apps))
	}
	if got["total"].(float64) != 0 {
		t.Errorf("total: got %v, want 0 (empty breakdown sums to 0)", got["total"])
	}
	if got["areaName"] != "" {
		t.Errorf("areaName: got %v, want empty when no applications", got["areaName"])
	}
	breakdown, ok := got["statusBreakdown"].([]any)
	if !ok {
		t.Fatalf("statusBreakdown must be a non-null array even when empty, got %T", got["statusBreakdown"])
	}
	if len(breakdown) != 0 {
		t.Errorf("statusBreakdown length: got %d, want 0", len(breakdown))
	}
}

func TestRecentHandler_NonIntIdReturns400(t *testing.T) {
	t.Parallel()
	store := &fakeRecentStore{apps: recentApps(t, 2)}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/abc/applications")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-int id: got %d, want 400", rec.Code)
	}
	if store.lastCap != 0 || store.breakdownCalled {
		t.Errorf("store must not be hit on a malformed id")
	}
}

func TestRecentHandler_StoreErrorReturns500(t *testing.T) {
	t.Parallel()
	store := &fakeRecentStore{err: context.DeadlineExceeded}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("store error: got %d, want 500", rec.Code)
	}
}

func TestRecentHandler_BreakdownErrorReturns500(t *testing.T) {
	t.Parallel()
	// The bounded read succeeds but the whole-partition breakdown fails -> 500 (no
	// partial total).
	store := &fakeRecentStore{apps: recentApps(t, 2), breakdownErr: context.DeadlineExceeded}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("breakdown error: got %d, want 500", rec.Code)
	}
}
