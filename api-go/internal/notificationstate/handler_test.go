package notificationstate

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

type fakeStateStore struct {
	states map[string]State
	unread int

	getErr      error
	cntErr      error
	markAllErr  error
	markReadErr error

	markAllCalls  int
	markReadCalls int
	markReadUIDs  []string
	markReadAuths []int
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

func (f *fakeStateStore) UnreadCount(_ context.Context, _ string) (int, error) {
	if f.cntErr != nil {
		return 0, f.cntErr
	}
	return f.unread, nil
}

func (f *fakeStateStore) MarkAllRead(_ context.Context, _ string, _ time.Time) (int64, error) {
	f.markAllCalls++
	if f.markAllErr != nil {
		return 0, f.markAllErr
	}
	return 0, nil
}

func (f *fakeStateStore) MarkApplicationsRead(_ context.Context, _ string, uids []string, authorityIDs []int, _ time.Time) (int64, error) {
	f.markReadCalls++
	f.markReadUIDs = uids
	f.markReadAuths = authorityIDs
	if f.markReadErr != nil {
		return 0, f.markReadErr
	}
	return int64(len(uids)), nil
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
	// DateTimeOffset wire format: numeric offset, never Z.
	want := `{"lastReadAt":"2026-06-01T12:00:00+00:00","version":3,"totalUnreadCount":7}`
	if rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body.String(), want)
	}
}

// TestHandler_Get_FirstTouchNoSeed proves GET is side-effect-free for a user
// with no state row: version 0, lastReadAt computed at now (not persisted), and
// no write to the store (ADR 0035 removed the first-touch watermark seed).
func TestHandler_Get_FirstTouchNoSeed(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 12, 9, 30, 0, 0, time.UTC)
	store := newFakeStateStore()

	rec := doReq(t, testMux(t, store, now), http.MethodGet, "/v1/me/notification-state", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := `{"lastReadAt":"2026-06-12T09:30:00+00:00","version":0,"totalUnreadCount":0}`
	if rec.Body.String() != want {
		t.Errorf("body = %s, want %s", rec.Body.String(), want)
	}
	// No write of any kind: GET must not seed a state row.
	if len(store.states) != 0 || store.markAllCalls != 0 || store.markReadCalls != 0 {
		t.Errorf("GET wrote state: states=%v markAll=%d markRead=%d",
			store.states, store.markAllCalls, store.markReadCalls)
	}
}

func TestHandler_MarkAllRead(t *testing.T) {
	t.Parallel()

	t.Run("clears and returns 204", func(t *testing.T) {
		t.Parallel()
		store := newFakeStateStore()

		rec := doReq(t, testMux(t, store, time.Now()), http.MethodPost, "/v1/me/notification-state/mark-all-read", "")
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		if store.markAllCalls != 1 {
			t.Errorf("MarkAllRead calls = %d, want 1", store.markAllCalls)
		}
	})

	t.Run("store error is 500", func(t *testing.T) {
		t.Parallel()
		store := newFakeStateStore()
		store.markAllErr = errors.New("db down")

		rec := doReq(t, testMux(t, store, time.Now()), http.MethodPost, "/v1/me/notification-state/mark-all-read", "")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
	})
}

func TestHandler_MarkRead(t *testing.T) {
	t.Parallel()

	const path = "/v1/me/applications/mark-read"

	t.Run("single application clears and returns 204", func(t *testing.T) {
		t.Parallel()
		store := newFakeStateStore()

		body := `{"applications":[{"applicationUid":"24-01234","authorityId":330}]}`
		rec := doReq(t, testMux(t, store, time.Now()), http.MethodPost, path, body)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		if store.markReadCalls != 1 {
			t.Fatalf("MarkApplicationsRead calls = %d, want 1", store.markReadCalls)
		}
		if !reflect.DeepEqual(store.markReadUIDs, []string{"24-01234"}) ||
			!reflect.DeepEqual(store.markReadAuths, []int{330}) {
			t.Errorf("passed uids=%v auths=%v, want [24-01234] [330]", store.markReadUIDs, store.markReadAuths)
		}
	})

	t.Run("several applications pass every pair", func(t *testing.T) {
		t.Parallel()
		store := newFakeStateStore()

		body := `{"applications":[` +
			`{"applicationUid":"24-01234","authorityId":330},` +
			`{"applicationUid":"24-05678","authorityId":331}]}`
		rec := doReq(t, testMux(t, store, time.Now()), http.MethodPost, path, body)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		if !reflect.DeepEqual(store.markReadUIDs, []string{"24-01234", "24-05678"}) ||
			!reflect.DeepEqual(store.markReadAuths, []int{330, 331}) {
			t.Errorf("passed uids=%v auths=%v", store.markReadUIDs, store.markReadAuths)
		}
	})

	t.Run("empty array marks nothing and returns 204", func(t *testing.T) {
		t.Parallel()
		store := newFakeStateStore()

		rec := doReq(t, testMux(t, store, time.Now()), http.MethodPost, path, `{"applications":[]}`)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rec.Code)
		}
		// The store is asked to clear the empty set — never "all".
		if store.markReadCalls != 1 || len(store.markReadUIDs) != 0 || len(store.markReadAuths) != 0 {
			t.Errorf("empty array: calls=%d uids=%v auths=%v, want 1 call with empty sets",
				store.markReadCalls, store.markReadUIDs, store.markReadAuths)
		}
	})

	t.Run("idempotent second call is still 204", func(t *testing.T) {
		t.Parallel()
		store := newFakeStateStore()
		mux := testMux(t, store, time.Now())
		body := `{"applications":[{"applicationUid":"24-01234","authorityId":330}]}`

		if rec := doReq(t, mux, http.MethodPost, path, body); rec.Code != http.StatusNoContent {
			t.Fatalf("first call status = %d, want 204", rec.Code)
		}
		if rec := doReq(t, mux, http.MethodPost, path, body); rec.Code != http.StatusNoContent {
			t.Fatalf("second call status = %d, want 204", rec.Code)
		}
	})

	t.Run("malformed body is a bodyless 400", func(t *testing.T) {
		t.Parallel()
		store := newFakeStateStore()

		rec := doReq(t, testMux(t, store, time.Now()), http.MethodPost, path, `{"applications":`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if store.markReadCalls != 0 {
			t.Errorf("malformed body must not touch the store: calls=%d", store.markReadCalls)
		}
	})

	t.Run("over the cap is a 400", func(t *testing.T) {
		t.Parallel()
		store := newFakeStateStore()

		var sb strings.Builder
		sb.WriteString(`{"applications":[`)
		for i := 0; i <= maxMarkReadApplications; i++ {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(`{"applicationUid":"u","authorityId":1}`)
		}
		sb.WriteString(`]}`)

		rec := doReq(t, testMux(t, store, time.Now()), http.MethodPost, path, sb.String())
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
		if store.markReadCalls != 0 {
			t.Errorf("over-cap must not touch the store: calls=%d", store.markReadCalls)
		}
	})

	t.Run("store error is 500", func(t *testing.T) {
		t.Parallel()
		store := newFakeStateStore()
		store.markReadErr = errors.New("db down")

		body := `{"applications":[{"applicationUid":"24-01234","authorityId":330}]}`
		rec := doReq(t, testMux(t, store, time.Now()), http.MethodPost, path, body)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}
	})
}

func TestHandler_StoreFailures(t *testing.T) {
	t.Parallel()

	markReadBody := `{"applications":[{"applicationUid":"24-01234","authorityId":330}]}`
	for _, tc := range []struct {
		name               string
		method, path, body string
		configure          func(*fakeStateStore)
	}{
		{"get", http.MethodGet, "/v1/me/notification-state", "", func(f *fakeStateStore) { f.getErr = errors.New("boom") }},
		{"mark-all-read", http.MethodPost, "/v1/me/notification-state/mark-all-read", "", func(f *fakeStateStore) { f.markAllErr = errors.New("boom") }},
		{"mark-read", http.MethodPost, "/v1/me/applications/mark-read", markReadBody, func(f *fakeStateStore) { f.markReadErr = errors.New("boom") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := newFakeStateStore()
			tc.configure(store)
			rec := doReq(t, testMux(t, store, time.Now()), tc.method, tc.path, tc.body)
			if rec.Code != http.StatusInternalServerError {
				t.Errorf("%s %s status = %d, want 500", tc.method, tc.path, rec.Code)
			}
		})
	}
}
