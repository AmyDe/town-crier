package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/polling"
)

type fakePollingSwitch struct {
	st   polling.SwitchState
	sets []bool
}

func (f *fakePollingSwitch) State(context.Context) (polling.SwitchState, error) { return f.st, nil }

func (f *fakePollingSwitch) Set(_ context.Context, enabled bool, reason string, at time.Time) (polling.SwitchState, error) {
	f.sets = append(f.sets, enabled)
	f.st = polling.SwitchState{Enabled: enabled, Source: polling.SwitchSourceSet, Reason: reason, UpdatedAt: &at}
	return f.st, nil
}

type fakePlanItCalls struct {
	latest *polling.PlanItCall
	from   time.Time
	count  int
}

func (f *fakePlanItCalls) Latest(context.Context) (*polling.PlanItCall, error) { return f.latest, nil }

func (f *fakePlanItCalls) CountBetween(_ context.Context, from, _ time.Time) (int, error) {
	f.from = from
	return f.count, nil
}

var pollingNow = time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC)

func newPollingMux(sw *fakePollingSwitch, calls *fakePlanItCalls) *http.ServeMux {
	mux := http.NewServeMux()
	PollingRoutes(mux, "k", sw, calls, func() time.Time { return pollingNow }, slog.New(slog.DiscardHandler))
	return mux
}

func pollingRequestTo(t *testing.T, mux *http.ServeMux, method, key, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, "/v1/admin/polling", strings.NewReader(body))
	if key != "" {
		req.Header.Set(adminKeyHeader, key)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out map[string]any
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode %q: %v", rec.Body.String(), err)
		}
	}
	return rec, out
}

func TestPolling_GetReportsDefaultAndLastCall(t *testing.T) {
	t.Parallel()
	status := 429
	sw := &fakePollingSwitch{st: polling.SwitchState{Enabled: true, Source: polling.SwitchSourceDefault}}
	calls := &fakePlanItCalls{latest: &polling.PlanItCall{At: pollingNow.Add(-time.Minute), Status: &status}, count: 42}

	rec, out := pollingRequestTo(t, newPollingMux(sw, calls), http.MethodGet, "k", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if out["enabled"] != true || out["source"] != "default" || out["updatedAt"] != nil {
		t.Fatalf("switch fields = %v", out)
	}
	if out["lastCallAt"] != "2026-09-30T19:59:00Z" || out["lastCallStatus"] != float64(429) || out["callsToday"] != float64(42) {
		t.Fatalf("call fields = %v", out)
	}
	if want, _ := polling.BudgetDay(pollingNow); !calls.from.Equal(want) {
		t.Fatalf("counted from %s, want budget day start %s", calls.from, want)
	}
}

func TestPolling_GetWithNoCalls(t *testing.T) {
	t.Parallel()
	_, out := pollingRequestTo(t, newPollingMux(&fakePollingSwitch{}, &fakePlanItCalls{}), http.MethodGet, "k", "")

	if out["lastCallAt"] != nil || out["lastCallStatus"] != nil || out["callsToday"] != float64(0) {
		t.Fatalf("out = %v", out)
	}
}

func TestPolling_PutSetsAndReturnsStoredState(t *testing.T) {
	t.Parallel()
	sw := &fakePollingSwitch{st: polling.SwitchState{Enabled: true, Source: polling.SwitchSourceDefault}}

	rec, out := pollingRequestTo(t, newPollingMux(sw, &fakePlanItCalls{}), http.MethodPut, "k", `{"enabled":false,"reason":"dev trial"}`)

	if rec.Code != http.StatusOK || len(sw.sets) != 1 || sw.sets[0] {
		t.Fatalf("status = %d sets = %v", rec.Code, sw.sets)
	}
	if out["enabled"] != false || out["source"] != "set" || out["reason"] != "dev trial" || out["updatedAt"] != "2026-09-30T20:00:00Z" {
		t.Fatalf("out = %v", out)
	}
}

func TestPolling_PutRejectsBadBody(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"reason":"no flag"}`, `not json`, `{"enabled":"yes"}`} {
		sw := &fakePollingSwitch{}
		rec, _ := pollingRequestTo(t, newPollingMux(sw, &fakePlanItCalls{}), http.MethodPut, "k", body)
		if rec.Code != http.StatusBadRequest || len(sw.sets) != 0 {
			t.Errorf("%s: status = %d sets = %v", body, rec.Code, sw.sets)
		}
	}
}

func TestPolling_RequiresAdminKey(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodGet, http.MethodPut} {
		for _, key := range []string{"", "wrong"} {
			sw := &fakePollingSwitch{}
			rec, _ := pollingRequestTo(t, newPollingMux(sw, &fakePlanItCalls{}), method, key, `{"enabled":true}`)
			if rec.Code != http.StatusUnauthorized || len(sw.sets) != 0 {
				t.Errorf("%s key %q: status = %d sets = %v", method, key, rec.Code, sw.sets)
			}
		}
	}
}
