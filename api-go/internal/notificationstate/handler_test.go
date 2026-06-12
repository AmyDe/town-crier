package notificationstate

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

type fakeStateStore struct {
	states  map[string]State
	unread  int
	getErr  error
	saveErr error
	cntErr  error
}

func newFakeStateStore() *fakeStateStore {
	return &fakeStateStore{states: map[string]State{}}
}

func (f *fakeStateStore) Get(_ context.Context, userID string) (*State, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	st, ok := f.states[userID]
	if !ok {
		return nil, nil //nolint:nilnil // mirrors the store contract: absent is the first-touch signal
	}
	return &st, nil
}

func (f *fakeStateStore) Save(_ context.Context, st State) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.states[st.UserID] = st
	return nil
}

func (f *fakeStateStore) UnreadCount(_ context.Context, _ string, _ time.Time) (int, error) {
	if f.cntErr != nil {
		return 0, f.cntErr
	}
	return f.unread, nil
}

func testMux(t *testing.T, store stateStore, now time.Time) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, store, func() time.Time { return now }, slog.New(slog.DiscardHandler))
	return mux
}

func doReq(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(auth.WithSubject(ctx, "auth0|ns1"), method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHandler_Get_ExistingState(t *testing.T) {
	t.Parallel()

	store := newFakeStateStore()
	store.states["auth0|ns1"] = State{
		UserID:     "auth0|ns1",
		LastReadAt: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		Version:    3,
	}
	store.unread = 7

	rec := doReq(t, testMux(t, store, time.Now()), http.MethodGet, "/v1/me/notification-state", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("content-type = %q", got)
	}
	// .NET DateTimeOffset wire format: numeric offset, never Z.
	want := `{"lastReadAt":"2026-06-01T12:00:00+00:00","version":3,"totalUnreadCount":7}`
	if rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body.String(), want)
	}
}

func TestHandler_Get_FirstTouchSeedsAndPersists(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 12, 9, 30, 0, 0, time.UTC)
	store := newFakeStateStore()

	rec := doReq(t, testMux(t, store, now), http.MethodGet, "/v1/me/notification-state", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := `{"lastReadAt":"2026-06-12T09:30:00+00:00","version":1,"totalUnreadCount":0}`
	if rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body.String(), want)
	}
	// The seed must persist so subsequent reads are idempotent.
	if st, ok := store.states["auth0|ns1"]; !ok || st.Version != 1 || !st.LastReadAt.Equal(now) {
		t.Errorf("seed not persisted: %+v", store.states)
	}
}

func TestHandler_MarkAllRead(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)

	t.Run("existing state bumps version", func(t *testing.T) {
		t.Parallel()
		store := newFakeStateStore()
		store.states["auth0|ns1"] = State{UserID: "auth0|ns1", LastReadAt: now.Add(-time.Hour), Version: 2}

		rec := doReq(t, testMux(t, store, now), http.MethodPost, "/v1/me/notification-state/mark-all-read", "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		st := store.states["auth0|ns1"]
		if !st.LastReadAt.Equal(now) || st.Version != 3 {
			t.Errorf("state = %+v, want lastReadAt=now version=3", st)
		}
	})

	t.Run("first touch seeds without extra bump", func(t *testing.T) {
		t.Parallel()
		store := newFakeStateStore()

		rec := doReq(t, testMux(t, store, now), http.MethodPost, "/v1/me/notification-state/mark-all-read", "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		st := store.states["auth0|ns1"]
		if !st.LastReadAt.Equal(now) || st.Version != 1 {
			t.Errorf("state = %+v, want seeded version=1", st)
		}
	})
}

func TestHandler_Advance(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	baseline := State{UserID: "auth0|ns1", LastReadAt: now.Add(-time.Hour), Version: 2}

	tests := []struct {
		name        string
		body        string
		wantStatus  int
		wantVersion int
	}{
		{"forward advance bumps", `{"asOf":"2026-06-12T09:45:00+00:00"}`, http.StatusNoContent, 3},
		{"stale asOf is a no-op", `{"asOf":"2026-06-12T08:00:00+00:00"}`, http.StatusNoContent, 2},
		{"boundary instant is a no-op", `{"asOf":"2026-06-12T09:00:00+00:00"}`, http.StatusNoContent, 2},
		{"Z suffix accepted on input", `{"asOf":"2026-06-12T09:50:00Z"}`, http.StatusNoContent, 3},
		{"malformed body", `{"asOf":`, http.StatusBadRequest, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := newFakeStateStore()
			store.states["auth0|ns1"] = baseline

			rec := doReq(t, testMux(t, store, now), http.MethodPost, "/v1/me/notification-state/advance", tc.body)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := store.states["auth0|ns1"].Version; got != tc.wantVersion {
				t.Errorf("version = %d, want %d", got, tc.wantVersion)
			}
		})
	}

	t.Run("first touch seeds at now then advances", func(t *testing.T) {
		t.Parallel()
		store := newFakeStateStore()

		// asOf earlier than the seed: persists the seed untouched (version 1).
		rec := doReq(t, testMux(t, store, now), http.MethodPost, "/v1/me/notification-state/advance", `{"asOf":"2020-01-01T00:00:00+00:00"}`)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		st := store.states["auth0|ns1"]
		if !st.LastReadAt.Equal(now) || st.Version != 1 {
			t.Errorf("state = %+v, want seed at now version=1", st)
		}
	})
}

func TestHandler_StoreFailures(t *testing.T) {
	t.Parallel()

	store := newFakeStateStore()
	store.getErr = errors.New("cosmos down")
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/v1/me/notification-state", ""},
		{http.MethodPost, "/v1/me/notification-state/mark-all-read", ""},
		{http.MethodPost, "/v1/me/notification-state/advance", `{"asOf":"2026-06-12T09:45:00+00:00"}`},
	} {
		rec := doReq(t, testMux(t, store, time.Now()), tc.method, tc.path, tc.body)
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s %s status = %d, want 500", tc.method, tc.path, rec.Code)
		}
	}
}
