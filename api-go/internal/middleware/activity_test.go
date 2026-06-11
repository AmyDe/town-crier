package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// fakeActivityRecorder records the (userID, time) pairs the middleware reports,
// and can be made to fail to prove failures are swallowed.
type fakeActivityRecorder struct {
	mu    sync.Mutex
	calls []activityCall
	err   error
}

type activityCall struct {
	userID string
	at     time.Time
}

func (f *fakeActivityRecorder) RecordActivity(_ context.Context, userID string, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, activityCall{userID: userID, at: at})
	return nil
}

func (f *fakeActivityRecorder) recorded() []activityCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]activityCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func TestRecordActivity_AuthenticatedRequestRecorded(t *testing.T) {
	t.Parallel()

	rec := &fakeActivityRecorder{}
	clock := &fixedClock{t: time.Unix(2000, 0)}
	mw := RecordActivity(rec, clock.now, slogDiscard())

	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, authedRequest("auth0|abc"))

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	calls := rec.recorded()
	if len(calls) != 1 || calls[0].userID != "auth0|abc" || !calls[0].at.Equal(clock.t) {
		t.Errorf("activity not recorded as expected: %+v", calls)
	}
}

func TestRecordActivity_AnonymousRequestSkipped(t *testing.T) {
	t.Parallel()

	rec := &fakeActivityRecorder{}
	clock := &fixedClock{t: time.Unix(2000, 0)}
	mw := RecordActivity(rec, clock.now, slogDiscard())

	r := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/health", nil)
	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, r)

	if len(rec.recorded()) != 0 {
		t.Errorf("anonymous request should not record activity: %+v", rec.recorded())
	}
}

func TestRecordActivity_FailureSwallowed(t *testing.T) {
	t.Parallel()

	rec := &fakeActivityRecorder{err: errors.New("cosmos down")}
	clock := &fixedClock{t: time.Unix(2000, 0)}
	mw := RecordActivity(rec, clock.now, slogDiscard())

	w := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(w, authedRequest("auth0|abc"))

	// A recorder failure must never turn a successful request into a 500.
	if w.Code != http.StatusOK {
		t.Errorf("recorder failure leaked to response: got %d, want 200", w.Code)
	}
}

func TestRecordActivity_RunsAfterHandler(t *testing.T) {
	t.Parallel()

	rec := &fakeActivityRecorder{}
	clock := &fixedClock{t: time.Unix(2000, 0)}
	mw := RecordActivity(rec, clock.now, slogDiscard())

	var orderedHandlerRan bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Activity must not have been recorded yet — it runs only after the
		// handler completes (mirroring .NET's `await next(); then record`).
		if len(rec.recorded()) != 0 {
			t.Error("activity recorded before handler completed")
		}
		orderedHandlerRan = true
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	mw(inner).ServeHTTP(w, authedRequest("auth0|abc"))

	if !orderedHandlerRan {
		t.Fatal("inner handler did not run")
	}
	if len(rec.recorded()) != 1 {
		t.Errorf("activity should be recorded once after the handler, got %d", len(rec.recorded()))
	}
}
