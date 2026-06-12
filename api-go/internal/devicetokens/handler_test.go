package devicetokens

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

type fakeRegStore struct {
	regs    map[string]DeviceRegistration // keyed by userID + "|" + token
	getErr  error
	saveErr error
	delErr  error
}

func newFakeRegStore() *fakeRegStore {
	return &fakeRegStore{regs: map[string]DeviceRegistration{}}
}

func (f *fakeRegStore) GetByToken(_ context.Context, userID, token string) (*DeviceRegistration, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	reg, ok := f.regs[userID+"|"+token]
	if !ok {
		return nil, nil //nolint:nilnil // mirrors the store contract: absent is not an error
	}
	return &reg, nil
}

func (f *fakeRegStore) Save(_ context.Context, reg DeviceRegistration) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.regs[reg.UserID+"|"+reg.Token] = reg
	return nil
}

func (f *fakeRegStore) Delete(_ context.Context, userID, token string) error {
	if f.delErr != nil {
		return f.delErr
	}
	delete(f.regs, userID+"|"+token)
	return nil
}

func testMux(t *testing.T, store registrationStore, now time.Time) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, store, func() time.Time { return now }, slog.New(slog.DiscardHandler))
	return mux
}

func doReq(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req := httptest.NewRequestWithContext(auth.WithSubject(ctx, "auth0|dev1"), method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHandler_Register(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 12, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"valid ios", `{"token":"tok-1","platform":"ios"}`, http.StatusNoContent},
		{"valid canonical case", `{"token":"tok-2","platform":"Android"}`, http.StatusNoContent},
		{"unknown platform", `{"token":"tok-3","platform":"spectrum"}`, http.StatusBadRequest},
		{"malformed json", `{"token":`, http.StatusBadRequest},
		{"blank token reaches the domain guard", `{"token":" ","platform":"ios"}`, http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := newFakeRegStore()
			rec := doReq(t, testMux(t, store, now), http.MethodPut, "/v1/me/device-token", tc.body)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if rec.Code == http.StatusNoContent && rec.Body.Len() != 0 {
				t.Errorf("204 must be bodyless, got %s", rec.Body.String())
			}
		})
	}
}

func TestHandler_Register_RefreshesExisting(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	refreshed := time.Date(2026, 6, 12, 8, 0, 0, 0, time.UTC)

	store := newFakeRegStore()
	store.regs["auth0|dev1|tok-1"] = DeviceRegistration{
		UserID: "auth0|dev1", Token: "tok-1", Platform: PlatformIos, RegisteredAt: created,
	}

	rec := doReq(t, testMux(t, store, refreshed), http.MethodPut, "/v1/me/device-token", `{"token":"tok-1","platform":"ios"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	got := store.regs["auth0|dev1|tok-1"]
	if !got.RegisteredAt.Equal(refreshed) {
		t.Errorf("RegisteredAt = %v, want refreshed %v", got.RegisteredAt, refreshed)
	}
}

func TestHandler_Register_StoreFailure(t *testing.T) {
	t.Parallel()

	store := newFakeRegStore()
	store.saveErr = errors.New("cosmos down")
	rec := doReq(t, testMux(t, store, time.Now()), http.MethodPut, "/v1/me/device-token", `{"token":"tok-1","platform":"ios"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}

func TestHandler_Remove(t *testing.T) {
	t.Parallel()

	store := newFakeRegStore()
	store.regs["auth0|dev1|tok-1"] = DeviceRegistration{UserID: "auth0|dev1", Token: "tok-1"}

	rec := doReq(t, testMux(t, store, time.Now()), http.MethodDelete, "/v1/me/device-token/tok-1", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if _, ok := store.regs["auth0|dev1|tok-1"]; ok {
		t.Error("registration should be removed")
	}

	// Idempotent: a second delete of the now-absent token is still 204.
	rec = doReq(t, testMux(t, store, time.Now()), http.MethodDelete, "/v1/me/device-token/tok-1", "")
	if rec.Code != http.StatusNoContent {
		t.Errorf("repeat delete status = %d, want 204", rec.Code)
	}
}
