package savedapplications

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

const user = "auth0|u"

type fakeSavedStore struct {
	rows       []SavedApplication
	exists     bool
	saved      []SavedApplication
	deletedUID []string
	listErr    error
}

func (f *fakeSavedStore) Save(_ context.Context, sa SavedApplication) error {
	f.saved = append(f.saved, sa)
	f.rows = append(f.rows, sa)
	return nil
}
func (f *fakeSavedStore) Exists(_ context.Context, _, _ string) (bool, error) { return f.exists, nil }
func (f *fakeSavedStore) Delete(_ context.Context, _, uid string) error {
	f.deletedUID = append(f.deletedUID, uid)
	return nil
}
func (f *fakeSavedStore) GetByUserID(_ context.Context, _ string) ([]SavedApplication, error) {
	return f.rows, f.listErr
}

type fakeApps struct {
	upserted []applications.PlanningApplication
}

func (f *fakeApps) Upsert(_ context.Context, a applications.PlanningApplication) error {
	f.upserted = append(f.upserted, a)
	return nil
}

func testMux(t *testing.T, store savedStore, apps appUpserter) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	now := func() time.Time { return time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC) }
	Routes(mux, store, apps, now, slog.New(slog.DiscardHandler))
	return mux
}

func doReq(t *testing.T, mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(auth.WithSubject(context.Background(), user), method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

const validSaveBody = `{"name":"24/0123/FUL","uid":"ABC-1","areaName":"City of London","areaId":471,"address":"1 St","description":"d","lastDifferent":"2026-03-02T09:30:00+00:00"}`

func TestHandler_Save_DualWriteAndCanonicalKey(t *testing.T) {
	t.Parallel()
	store := &fakeSavedStore{exists: false}
	apps := &fakeApps{}
	rec := doReq(t, testMux(t, store, apps), http.MethodPut, "/v1/me/saved-applications/whatever-path-uid", validSaveBody)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204 (body %s)", rec.Code, rec.Body)
	}
	if len(apps.upserted) != 1 || apps.upserted[0].Name != "24/0123/FUL" {
		t.Errorf("master record not upserted: %+v", apps.upserted)
	}
	if len(store.saved) != 1 || store.saved[0].ApplicationUID != "471/24/0123/FUL" {
		t.Errorf("saved row not keyed on canonical uid: %+v", store.saved)
	}
}

func TestHandler_Save_IdempotentWhenAlreadySaved(t *testing.T) {
	t.Parallel()
	store := &fakeSavedStore{exists: true}
	apps := &fakeApps{}
	rec := doReq(t, testMux(t, store, apps), http.MethodPut, "/v1/me/saved-applications/x", validSaveBody)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d", rec.Code)
	}
	// Master record still upserted, but no duplicate saved row written.
	if len(apps.upserted) != 1 {
		t.Errorf("master upsert should still run: %+v", apps.upserted)
	}
	if len(store.saved) != 0 {
		t.Errorf("must not re-save when already saved: %+v", store.saved)
	}
}

func TestHandler_Save_MissingUidOrName(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"missing uid":  `{"name":"n","uid":"","areaId":1,"areaName":"a","address":"x","description":"d","lastDifferent":"2026-03-02T09:30:00+00:00"}`,
		"missing name": `{"name":" ","uid":"u","areaId":1,"areaName":"a","address":"x","description":"d","lastDifferent":"2026-03-02T09:30:00+00:00"}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := &fakeSavedStore{}
			rec := doReq(t, testMux(t, store, &fakeApps{}), http.MethodPut, "/v1/me/saved-applications/x", body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d, want 400", rec.Code)
			}
			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got["error"] != invalidBodyMessage {
				t.Errorf("error: got %v", got["error"])
			}
			if v, ok := got["message"]; !ok || v != nil {
				t.Errorf("message must be present and null: %v", v)
			}
			if len(store.saved) != 0 {
				t.Error("invalid save must not persist")
			}
		})
	}
}

func TestHandler_Delete(t *testing.T) {
	t.Parallel()
	store := &fakeSavedStore{}
	// The catch-all captures a slash-bearing canonical uid whole.
	rec := doReq(t, testMux(t, store, &fakeApps{}), http.MethodDelete, "/v1/me/saved-applications/471/24/0123/FUL", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204", rec.Code)
	}
	if len(store.deletedUID) != 1 || store.deletedUID[0] != "471/24/0123/FUL" {
		t.Errorf("deleted uid: %+v", store.deletedUID)
	}
}

func TestHandler_List(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)
	store := &fakeSavedStore{rows: []SavedApplication{NewSavedApplication(user, testApp(t), now)}}
	rec := doReq(t, testMux(t, store, &fakeApps{}), http.MethodGet, "/v1/me/saved-applications", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode (must be a JSON array): %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0]["applicationUid"] != "471/24/0123/FUL" {
		t.Errorf("applicationUid: %v", got[0]["applicationUid"])
	}
	app, ok := got[0]["application"].(map[string]any)
	if !ok || app["uid"] != "ABC-24-0123" {
		t.Errorf("embedded application not rendered: %+v", got[0]["application"])
	}
}

func TestHandler_List_EmptyArray(t *testing.T) {
	t.Parallel()
	rec := doReq(t, testMux(t, &fakeSavedStore{}, &fakeApps{}), http.MethodGet, "/v1/me/saved-applications", "")
	if body := strings.TrimSpace(rec.Body.String()); body != `[]` {
		t.Errorf("empty list must be [], got %s", body)
	}
}

func TestHandler_List_SkipsNilSnapshotRows(t *testing.T) {
	t.Parallel()
	// A legacy row with no embedded snapshot is skipped (backfill deferred to tc-wans).
	store := &fakeSavedStore{rows: []SavedApplication{{UserID: user, ApplicationUID: "legacy", AuthorityID: 1, Application: nil}}}
	rec := doReq(t, testMux(t, store, &fakeApps{}), http.MethodGet, "/v1/me/saved-applications", "")
	if body := strings.TrimSpace(rec.Body.String()); body != `[]` {
		t.Errorf("nil-snapshot row must be skipped, got %s", body)
	}
}
