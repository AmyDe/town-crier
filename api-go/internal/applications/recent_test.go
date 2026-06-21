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
// honours cap the way Cosmos TOP @cap does, so the handler's cap/limit logic is
// exercised against realistic bounded results.
type fakeRecentStore struct {
	apps []PlanningApplication
	err  error

	lastAuthorityCode string
	lastCap           int
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
	store := &fakeRecentStore{apps: recentApps(t, 2)}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}
	// The handler reads the bounded cap (200), scoped to the numeric id -> code.
	if store.lastAuthorityCode != "471" || store.lastCap != 200 {
		t.Errorf("store call: authorityCode=%q cap=%d, want \"471\" 200", store.lastAuthorityCode, store.lastCap)
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
	if got["total"].(float64) != 2 {
		t.Errorf("total: got %v, want 2", got["total"])
	}
	if got["totalCapped"].(bool) {
		t.Errorf("totalCapped: got true, want false")
	}
	// The wire shape of an application carries the SEO-relevant fields.
	first := apps[0].(map[string]any)
	for _, key := range []string{"uid", "name", "address", "description", "appState", "startDate", "link", "url"} {
		if _, present := first[key]; !present {
			t.Errorf("application missing field %q: %v", key, first)
		}
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
	if store.lastCap != 0 {
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
	if store.lastCap != 0 {
		t.Errorf("store must not be hit when the configured key is empty")
	}
}

func TestRecentHandler_LimitsReturnedApplications(t *testing.T) {
	t.Parallel()
	store := &fakeRecentStore{apps: recentApps(t, 50)}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications?limit=10")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	got := recentBody(t, rec)
	apps := got["applications"].([]any)
	if len(apps) != 10 {
		t.Errorf("applications length: got %d, want 10 (the limit)", len(apps))
	}
	// total reflects the full bounded read, not the rendered slice.
	if got["total"].(float64) != 50 {
		t.Errorf("total: got %v, want 50", got["total"])
	}
}

func TestRecentHandler_DefaultLimitIs30(t *testing.T) {
	t.Parallel()
	store := &fakeRecentStore{apps: recentApps(t, 50)}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications")

	got := recentBody(t, rec)
	apps := got["applications"].([]any)
	if len(apps) != 30 {
		t.Errorf("applications length: got %d, want 30 (default limit)", len(apps))
	}
}

func TestRecentHandler_LimitHardCappedAt100(t *testing.T) {
	t.Parallel()
	store := &fakeRecentStore{apps: recentApps(t, 150)}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications?limit=999")

	got := recentBody(t, rec)
	apps := got["applications"].([]any)
	if len(apps) != 100 {
		t.Errorf("applications length: got %d, want 100 (hard max limit)", len(apps))
	}
}

func TestRecentHandler_TotalCappedWhenReadHitsCap(t *testing.T) {
	t.Parallel()
	// Exactly cap (200) documents available -> the bounded read returns cap.
	store := &fakeRecentStore{apps: recentApps(t, 200)}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/471/applications")

	got := recentBody(t, rec)
	if got["total"].(float64) != 200 {
		t.Errorf("total: got %v, want 200", got["total"])
	}
	if !got["totalCapped"].(bool) {
		t.Errorf("totalCapped: got false, want true when the read hits cap")
	}
	// Still renders only the default limit.
	if apps := got["applications"].([]any); len(apps) != 30 {
		t.Errorf("applications length: got %d, want 30", len(apps))
	}
}

func TestRecentHandler_EmptyResultIsNonNullArray(t *testing.T) {
	t.Parallel()
	store := &fakeRecentStore{apps: nil}

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
		t.Errorf("total: got %v, want 0", got["total"])
	}
	if got["areaName"] != "" {
		t.Errorf("areaName: got %v, want empty when no applications", got["areaName"])
	}
}

func TestRecentHandler_NonIntIdReturns400(t *testing.T) {
	t.Parallel()
	store := &fakeRecentStore{apps: recentApps(t, 2)}

	rec := serveRecent(t, store, "buildkey", "buildkey", "/v1/authorities/abc/applications")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("non-int id: got %d, want 400", rec.Code)
	}
	if store.lastCap != 0 {
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
