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

type refreshCall struct {
	userID string
	app    PlanningApplication
}

type fakeRefresher struct {
	calls []refreshCall
	err   error
}

func (f *fakeRefresher) RefreshSnapshot(_ context.Context, userID string, app PlanningApplication) error {
	f.calls = append(f.calls, refreshCall{userID: userID, app: app})
	return f.err
}

// serveGet drives the read endpoint with a refresher absent (the nil-safe path)
// and an authenticated subject.
func serveGet(t *testing.T, store appStore, path string) *httptest.ResponseRecorder {
	t.Helper()
	return serveGetWith(t, store, nil, "auth0|u", path)
}

func serveGetWith(t *testing.T, store appStore, refresher snapshotRefresher, subject, path string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	Routes(mux, store, refresher, slog.New(slog.DiscardHandler))
	ctx := context.Background()
	if subject != "" {
		ctx = auth.WithSubject(ctx, subject)
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
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

func TestHandler_GetByAuthorityAndName_RefreshesOnTap(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	refresher := &fakeRefresher{}
	rec := serveGetWith(t, &fakeAppStore{app: a, found: true}, refresher, "auth0|u", "/v1/applications/471/24/0123/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if len(refresher.calls) != 1 {
		t.Fatalf("expected one refresh call, got %d", len(refresher.calls))
	}
	if refresher.calls[0].userID != "auth0|u" || refresher.calls[0].app.UID != a.UID {
		t.Errorf("refresh call: %+v", refresher.calls[0])
	}
}

func TestHandler_GetByAuthorityAndName_RefreshFailureDoesNotFailRead(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	refresher := &fakeRefresher{err: context.DeadlineExceeded}
	rec := serveGetWith(t, &fakeAppStore{app: a, found: true}, refresher, "auth0|u", "/v1/applications/471/24/0123/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("refresh error must not fail the read: got %d, want 200", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("body must still be written on refresh failure")
	}
}

func TestHandler_GetByAuthorityAndName_NoRefreshWhenNotFound(t *testing.T) {
	t.Parallel()
	refresher := &fakeRefresher{}
	rec := serveGetWith(t, &fakeAppStore{found: false}, refresher, "auth0|u", "/v1/applications/471/missing")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if len(refresher.calls) != 0 {
		t.Errorf("must not refresh a missing application: %+v", refresher.calls)
	}
}

func TestHandler_GetByAuthorityAndName_NoRefreshWhenAnonymous(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	refresher := &fakeRefresher{}
	rec := serveGetWith(t, &fakeAppStore{app: a, found: true}, refresher, "", "/v1/applications/471/24/0123/FUL")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if len(refresher.calls) != 0 {
		t.Errorf("must not refresh without an authenticated subject: %+v", refresher.calls)
	}
}
