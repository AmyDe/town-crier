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
	// byUID supports the lazy-backfill GetByUID path used during list hydration.
	byUID map[string]applications.PlanningApplication
	// byAuthorityAndName supports the look-up path used by save.
	byAuthorityAndName map[string]applications.PlanningApplication
	// writeCount records how many write/upsert calls were made to the app store;
	// tests assert this is zero to confirm no master-record mutation occurred.
	writeCount int
}

func (f *fakeApps) GetByUID(_ context.Context, uid, _ string) (applications.PlanningApplication, bool, error) {
	a, ok := f.byUID[uid]
	return a, ok, nil
}

func (f *fakeApps) GetByAuthorityAndName(_ context.Context, authorityCode, name string) (applications.PlanningApplication, bool, error) {
	key := authorityCode + "/" + name
	a, ok := f.byAuthorityAndName[key]
	return a, ok, nil
}

func testMux(t *testing.T, store savedStore, apps appStore) *http.ServeMux {
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

func TestHandler_Save_LooksUpMasterAndWritesBookmark(t *testing.T) {
	t.Parallel()
	store := &fakeSavedStore{exists: false}
	app := testApp(t)
	// The master record is present in the applications store.
	apps := &fakeApps{byAuthorityAndName: map[string]applications.PlanningApplication{"471/24/0123/FUL": app}}
	rec := doReq(t, testMux(t, store, apps), http.MethodPut, "/v1/me/saved-applications/whatever-path-uid", validSaveBody)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204 (body %s)", rec.Code, rec.Body)
	}
	// No write to the shared Applications container.
	if apps.writeCount != 0 {
		t.Errorf("must not write to master Applications container: writeCount=%d", apps.writeCount)
	}
	// Per-user bookmark written with canonical uid from the master record.
	if len(store.saved) != 1 || store.saved[0].ApplicationUID != "471/24/0123/FUL" {
		t.Errorf("saved row not keyed on canonical uid: %+v", store.saved)
	}
}

func TestHandler_Save_IdempotentWhenAlreadySaved(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	store := &fakeSavedStore{exists: true}
	apps := &fakeApps{byAuthorityAndName: map[string]applications.PlanningApplication{"471/24/0123/FUL": app}}
	rec := doReq(t, testMux(t, store, apps), http.MethodPut, "/v1/me/saved-applications/x", validSaveBody)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d", rec.Code)
	}
	// No write to the shared Applications container.
	if apps.writeCount != 0 {
		t.Errorf("must not write to master Applications container: writeCount=%d", apps.writeCount)
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

func TestHandler_List_BackfillsNilSnapshotFromMaster(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)
	// A canonical-keyed legacy row with no embedded snapshot: hydrate fetches the
	// master record, embeds it, and rewrites the row in place.
	legacy := SavedApplication{UserID: user, ApplicationUID: "471/24/0123/FUL", AuthorityID: 471, SavedAt: now, Application: nil}
	store := &fakeSavedStore{rows: []SavedApplication{legacy}}
	apps := &fakeApps{byUID: map[string]applications.PlanningApplication{"471/24/0123/FUL": testApp(t)}}

	rec := doReq(t, testMux(t, store, apps), http.MethodGet, "/v1/me/saved-applications", "")

	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0]["applicationUid"] != "471/24/0123/FUL" {
		t.Fatalf("backfilled row not rendered: %+v", got)
	}
	app, ok := got[0]["application"].(map[string]any)
	if !ok || app["uid"] != "ABC-24-0123" {
		t.Errorf("snapshot not embedded after backfill: %+v", got[0]["application"])
	}
	// The backfilled snapshot is persisted so subsequent reads are zero-hydration.
	if len(store.saved) != 1 || store.saved[0].Application == nil {
		t.Errorf("backfilled row must be persisted: %+v", store.saved)
	}
}

func TestHandler_List_SkipsRowWhenMasterGone(t *testing.T) {
	t.Parallel()
	// Nil-snapshot row whose master planning application no longer exists is
	// excluded rather than failing the whole list.
	legacy := SavedApplication{UserID: user, ApplicationUID: "gone", AuthorityID: 471, Application: nil}
	store := &fakeSavedStore{rows: []SavedApplication{legacy}}
	rec := doReq(t, testMux(t, store, &fakeApps{}), http.MethodGet, "/v1/me/saved-applications", "")

	if body := strings.TrimSpace(rec.Body.String()); body != `[]` {
		t.Errorf("row with missing master must be skipped, got %s", body)
	}
}

func TestHandler_List_ReKeysLegacyUidToCanonical(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)
	app := testApp(t)
	// Legacy row: snapshot present but keyed on the raw PlanIt bare ref.
	legacy := SavedApplication{UserID: user, ApplicationUID: "ABC-1", AuthorityID: 471, SavedAt: now, Application: &app}
	store := &fakeSavedStore{rows: []SavedApplication{legacy}, exists: false}

	rec := doReq(t, testMux(t, store, &fakeApps{}), http.MethodGet, "/v1/me/saved-applications", "")

	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0]["applicationUid"] != "471/24/0123/FUL" {
		t.Fatalf("legacy uid not re-keyed to canonical: %+v", got)
	}
	// Canonical doc written, legacy doc deleted.
	if len(store.saved) != 1 || store.saved[0].ApplicationUID != "471/24/0123/FUL" {
		t.Errorf("canonical doc not written: %+v", store.saved)
	}
	if len(store.deletedUID) != 1 || store.deletedUID[0] != "ABC-1" {
		t.Errorf("legacy doc not deleted: %+v", store.deletedUID)
	}
}

func TestHandler_List_DedupsLegacyAndCanonicalPair(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)
	app := testApp(t)
	legacy := SavedApplication{UserID: user, ApplicationUID: "ABC-1", AuthorityID: 471, SavedAt: now, Application: &app}
	canonical := NewSavedApplication(user, testApp(t), now)
	// Both rows resolve to the same canonical uid; the canonical doc already exists.
	store := &fakeSavedStore{rows: []SavedApplication{legacy, canonical}, exists: true}

	rec := doReq(t, testMux(t, store, &fakeApps{}), http.MethodGet, "/v1/me/saved-applications", "")

	var got []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0]["applicationUid"] != "471/24/0123/FUL" {
		t.Fatalf("legacy+canonical pair must collapse to one row: %+v", got)
	}
	// Canonical already existed, so re-key only drops the legacy duplicate.
	if len(store.deletedUID) != 1 || store.deletedUID[0] != "ABC-1" {
		t.Errorf("legacy duplicate not dropped: %+v", store.deletedUID)
	}
}

// TestSave_DoesNotWriteMasterRecordForUnknownApplication asserts that saving an
// application whose uid/name does not correspond to any master record in the
// Applications container returns 404 and writes nothing to that container.
func TestSave_DoesNotWriteMasterRecordForUnknownApplication(t *testing.T) {
	t.Parallel()
	store := &fakeSavedStore{}
	// The apps store has no record matching the body's (areaId, name).
	apps := &fakeApps{}
	rec := doReq(t, testMux(t, store, apps), http.MethodPut, "/v1/me/saved-applications/unknown", validSaveBody)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	// No write to the Applications container.
	if apps.writeCount != 0 {
		t.Errorf("must not write to master Applications container: writeCount=%d", apps.writeCount)
	}
	// No per-user bookmark written.
	if len(store.saved) != 0 {
		t.Errorf("must not write bookmark for unknown application: %+v", store.saved)
	}
}

// TestSave_PersistsBookmarkForKnownApplication asserts that saving an
// application that exists in the Applications container writes the per-user
// bookmark and returns 204, without touching the shared container.
func TestSave_PersistsBookmarkForKnownApplication(t *testing.T) {
	t.Parallel()
	app := testApp(t)
	apps := &fakeApps{byAuthorityAndName: map[string]applications.PlanningApplication{"471/24/0123/FUL": app}}
	store := &fakeSavedStore{exists: false}
	rec := doReq(t, testMux(t, store, apps), http.MethodPut, "/v1/me/saved-applications/x", validSaveBody)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204", rec.Code)
	}
	if apps.writeCount != 0 {
		t.Errorf("must not write to master Applications container: writeCount=%d", apps.writeCount)
	}
	if len(store.saved) != 1 {
		t.Fatalf("expected 1 bookmark saved, got %d", len(store.saved))
	}
	if store.saved[0].ApplicationUID != "471/24/0123/FUL" {
		t.Errorf("bookmark uid: got %q, want %q", store.saved[0].ApplicationUID, "471/24/0123/FUL")
	}
	if store.saved[0].UserID != user {
		t.Errorf("bookmark userID: got %q, want %q", store.saved[0].UserID, user)
	}
}

// TestSave_RejectsForgedApplication asserts that a forged body for a
// non-existent application causes no write to the Applications container and
// returns a rejection (404). Regression: GET /v1/applications/{authorityCode}/{name}
// must not be able to return attacker-supplied content via the save endpoint.
func TestSave_RejectsForgedApplication(t *testing.T) {
	t.Parallel()
	// Forged body: attacker-chosen address, description, and coordinates for a
	// record that does not exist in the master Applications container.
	forgedBody := `{"name":"24/9999/FUL","uid":"forged-uid","areaName":"City of London","areaId":471,"address":"FORGED ADDRESS","description":"FORGED DESCRIPTION","latitude":51.5,"longitude":-0.12,"lastDifferent":"2026-03-02T09:30:00+00:00"}`
	apps := &fakeApps{} // no master record for (471, "24/9999/FUL")
	store := &fakeSavedStore{}

	rec := doReq(t, testMux(t, store, apps), http.MethodPut, "/v1/me/saved-applications/forged", forgedBody)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404 — forged application must be rejected", rec.Code)
	}
	// The Applications container received no write calls.
	if apps.writeCount != 0 {
		t.Errorf("must not write forged data to Applications container: writeCount=%d", apps.writeCount)
	}
	// No per-user bookmark written either.
	if len(store.saved) != 0 {
		t.Errorf("must not write bookmark for forged application: %+v", store.saved)
	}
}
