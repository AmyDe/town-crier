package profiles

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// prefsUser is the subject under test for the zone-preferences handlers; the
// route's actual URL is irrelevant since the handlers read the path value and
// the context subject, not r.URL.Path.
const (
	prefsUser = "auth0|u"
	prefsZone = "z1"
)

func withZonePath(method, sub, body string) *http.Request {
	r := withSubject(method, "/v1/me/watch-zones/"+prefsZone+"/preferences", sub, body)
	r.SetPathValue("zoneId", prefsZone)
	return r
}

func seededProfile(t *testing.T, store *fakeStore) *UserProfile {
	t.Helper()
	p, err := NewProfile(prefsUser, "", time.Now())
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	store.byID[prefsUser] = p
	return p
}

func TestHandler_GetZonePreferences_DefaultsForNewZone(t *testing.T) {
	t.Parallel()
	store := newFakeStore()
	seededProfile(t, store)
	h := newTestHandler(store, newFakeAuth0())

	rec := httptest.NewRecorder()
	h.getZonePreferences(rec, withZonePath(http.MethodGet, prefsUser, ""))

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
	p := seededProfile(t, store)
	p.SetZonePreferences("z1", ZonePreferences{NewApplicationPush: false, NewApplicationEmail: true, DecisionPush: false, DecisionEmail: true})
	h := newTestHandler(store, newFakeAuth0())

	rec := httptest.NewRecorder()
	h.getZonePreferences(rec, withZonePath(http.MethodGet, prefsUser, ""))

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
	h := newTestHandler(newFakeStore(), newFakeAuth0())
	rec := httptest.NewRecorder()
	h.getZonePreferences(rec, withZonePath(http.MethodGet, "auth0|missing", ""))
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
	seededProfile(t, store)
	h := newTestHandler(store, newFakeAuth0())

	body := `{"newApplicationPush":false,"newApplicationEmail":true,"decisionPush":false,"decisionEmail":true}`
	rec := httptest.NewRecorder()
	h.putZonePreferences(rec, withZonePath(http.MethodPut, prefsUser, body))

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
	stored := store.byID[prefsUser].GetZonePreferences("z1")
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
	seededProfile(t, store)
	h := newTestHandler(store, newFakeAuth0())

	// Only one field present; the rest default to false (JSON zero-value
	// decoding for non-nullable bool fields).
	rec := httptest.NewRecorder()
	h.putZonePreferences(rec, withZonePath(http.MethodPut, prefsUser, `{"newApplicationPush":true}`))

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
	h := newTestHandler(newFakeStore(), newFakeAuth0())
	rec := httptest.NewRecorder()
	h.putZonePreferences(rec, withZonePath(http.MethodPut, "auth0|missing", `{"decisionPush":true}`))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d, want 404", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("404 must be bodyless, got %s", rec.Body)
	}
}
