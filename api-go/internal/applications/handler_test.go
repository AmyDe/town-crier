package applications

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

type fakeAppStore struct {
	app   PlanningApplication
	found bool
	err   error

	lastAuthorityCode string
	lastName          string
}

func (f *fakeAppStore) GetByAuthorityAndName(_ context.Context, authorityCode, name string) (PlanningApplication, bool, error) {
	f.lastAuthorityCode = authorityCode
	f.lastName = name
	return f.app, f.found, f.err
}

func serveGet(t *testing.T, store appStore, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, store, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(auth.WithSubject(context.Background(), "auth0|u"), http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestHandler_GetByAuthorityAndName_Found(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	store := &fakeAppStore{app: a, found: true}

	rec := serveGet(t, store, "/v1/applications/471/24/0123/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}
	// The {name...} wildcard captures the slash-bearing case reference whole.
	if store.lastAuthorityCode != "471" || store.lastName != "24/0123/FUL" {
		t.Errorf("routing: authorityCode=%q name=%q", store.lastAuthorityCode, store.lastName)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["uid"] != a.UID || got["areaId"].(float64) != float64(a.AreaID) {
		t.Errorf("body: %+v", got)
	}
	// Flat coordinates on the wire (no GeoJSON) and an explicit null unread event.
	if got["longitude"].(float64) != *a.Longitude {
		t.Errorf("longitude: got %v", got["longitude"])
	}
	if v, ok := got["latestUnreadEvent"]; !ok || v != nil {
		t.Errorf("latestUnreadEvent must be present and null: %v (present=%v)", v, ok)
	}
}

func TestHandler_GetByAuthorityAndName_NotFound(t *testing.T) {
	t.Parallel()
	rec := serveGet(t, &fakeAppStore{found: false}, "/v1/applications/471/missing")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("404 must be bodyless, got %s", rec.Body)
	}
}

func TestHandler_GetByAuthorityAndName_StoreError(t *testing.T) {
	t.Parallel()
	rec := serveGet(t, &fakeAppStore{err: context.DeadlineExceeded}, "/v1/applications/471/x")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}
