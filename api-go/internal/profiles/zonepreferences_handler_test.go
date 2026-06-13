package profiles

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func withZonePath(method, target, sub, zoneID, body string) *http.Request {
	r := withSubject(method, target, sub, body)
	r.SetPathValue("zoneId", zoneID)
	return r
}

func seededProfile(t *testing.T, store *fakeStore, userID string) *UserProfile {
	t.Helper()
	p, err := NewProfile(userID, "", time.Now())
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	store.byID[userID] = p
	return p
}

func TestHandler_GetZonePreferences_DefaultsForNewZone(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	seededProfile(t, store, "auth0|u")
	h := newTestHandler(store, newFakeAuth0(), "")

	rec := httptest.NewRecorder()
	h.getZonePreferences(rec, withZonePath(http.MethodGet, "/v1/me/watch-zones/z1/preferences", "auth0|u", "z1", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["zoneId"] != "z1" {
		t.Errorf("zoneId: got %v", got["zoneId"])
	}
	for _, k := range []string{"newApplicationPush", "newApplicationEmail", "decisionPush", "decisionEmail"} {
		if got[k] != true {
			t.Errorf("%s: got %v, want true (default)", k, got[k])
		}
	}
}

func TestHandler_GetZonePreferences_ReflectsStored(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	p := seededProfile(t, store, "auth0|u")
	p.SetZonePreferences("z1", ZonePreferences{NewApplicationPush: false, NewApplicationEmail: true, DecisionPush: false, DecisionEmail: true})
	h := newTestHandler(store, newFakeAuth0(), "")

	rec := httptest.NewRecorder()
	h.getZonePreferences(rec, withZonePath(http.MethodGet, "/v1/me/watch-zones/z1/preferences", "auth0|u", "z1", ""))

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["newApplicationPush"] != false || got["decisionPush"] != false {
		t.Errorf("stored prefs not reflected: %+v", got)
	}
	if got["newApplicationEmail"] != true || got["decisionEmail"] != true {
		t.Errorf("stored prefs not reflected: %+v", got)
	}
}

func TestHandler_GetZonePreferences_ProfileNotFound(t *testing.T) {
	t.Parallel()
	h := newTestHandler(newFakeStore(), newFakeAuth0(), "")
	rec := httptest.NewRecorder()
	h.getZonePreferences(rec, withZonePath(http.MethodGet, "/v1/me/watch-zones/z1/preferences", "auth0|missing", "z1", ""))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("404 must be bodyless, got %s", rec.Body)
	}
}

func TestHandler_PutZonePreferences_PersistsAndReturns(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	seededProfile(t, store, "auth0|u")
	h := newTestHandler(store, newFakeAuth0(), "")

	body := `{"newApplicationPush":false,"newApplicationEmail":true,"decisionPush":false,"decisionEmail":true}`
	rec := httptest.NewRecorder()
	h.putZonePreferences(rec, withZonePath(http.MethodPut, "/v1/me/watch-zones/z1/preferences", "auth0|u", "z1", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["zoneId"] != "z1" || got["newApplicationPush"] != false || got["decisionEmail"] != true {
		t.Errorf("response mismatch: %+v", got)
	}
	// Persisted onto the profile.
	stored := store.byID["auth0|u"].GetZonePreferences("z1")
	if stored.NewApplicationPush != false || stored.DecisionEmail != true {
		t.Errorf("prefs not persisted: %+v", stored)
	}
	if len(store.saved) != 1 {
		t.Errorf("expected one save, got %d", len(store.saved))
	}
}

func TestHandler_PutZonePreferences_MissingFieldsDefaultFalse(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	seededProfile(t, store, "auth0|u")
	h := newTestHandler(store, newFakeAuth0(), "")

	// Only one field present; the rest default to false (matching STJ on the
	// .NET command record's non-nullable bools).
	rec := httptest.NewRecorder()
	h.putZonePreferences(rec, withZonePath(http.MethodPut, "/v1/me/watch-zones/z1/preferences", "auth0|u", "z1", `{"newApplicationPush":true}`))

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["newApplicationPush"] != true {
		t.Errorf("newApplicationPush: got %v, want true", got["newApplicationPush"])
	}
	for _, k := range []string{"newApplicationEmail", "decisionPush", "decisionEmail"} {
		if got[k] != false {
			t.Errorf("%s: got %v, want false (omitted)", k, got[k])
		}
	}
}

func TestHandler_PutZonePreferences_ProfileNotFound(t *testing.T) {
	t.Parallel()
	h := newTestHandler(newFakeStore(), newFakeAuth0(), "")
	rec := httptest.NewRecorder()
	h.putZonePreferences(rec, withZonePath(http.MethodPut, "/v1/me/watch-zones/z1/preferences", "auth0|missing", "z1", `{"decisionPush":true}`))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("404 must be bodyless, got %s", rec.Body)
	}
}
