package profiles

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
)

// fakeStore is a hand-written profile store for handler tests.
type fakeStore struct {
	byID      map[string]*UserProfile
	saveErr   error
	getErr    error
	deleteErr error
	saved     []*UserProfile
	deleted   []string
}

func newFakeStore() *fakeStore { return &fakeStore{byID: map[string]*UserProfile{}} }

func (f *fakeStore) Get(_ context.Context, userID string) (*UserProfile, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	p, ok := f.byID[userID]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (f *fakeStore) Save(_ context.Context, p *UserProfile) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.byID[p.UserID] = p
	f.saved = append(f.saved, p)
	return nil
}

func (f *fakeStore) Delete(_ context.Context, userID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.byID, userID)
	f.deleted = append(f.deleted, userID)
	return nil
}

// fakeAuth0 records management calls for assertion.
type fakeAuth0 struct {
	tierUpdates  map[string]string
	deletedUsers []string
	updateErr    error
	deleteErr    error
}

func newFakeAuth0() *fakeAuth0 { return &fakeAuth0{tierUpdates: map[string]string{}} }

func (f *fakeAuth0) UpdateSubscriptionTier(_ context.Context, userID, tier string) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.tierUpdates[userID] = tier
	return nil
}

func (f *fakeAuth0) DeleteUser(_ context.Context, userID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedUsers = append(f.deletedUsers, userID)
	return nil
}

func newTestHandler(store profileStore, a0 Auth0Manager, proDomains string) *handler {
	now := func() time.Time { return time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC) }
	return newHandler(store, a0, proDomains, now, slog.New(slog.DiscardHandler))
}

// withSubject builds a request carrying an authenticated subject in context, as
// the auth middleware would after validating the bearer token.
func withSubject(method, target, sub, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	r := httptest.NewRequestWithContext(context.Background(), method, target, reader)
	return r.WithContext(auth.WithSubject(r.Context(), sub))
}

func TestHandler_CreateProfile_ReturnsOkCamelCase(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	h := newTestHandler(store, newFakeAuth0(), "")

	rec := httptest.NewRecorder()
	h.create(rec, withSubject(http.MethodPost, "/v1/me", "auth0|new", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("content-type: got %q", ct)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// CreateUserProfileResult: { userId, pushEnabled, tier } in camelCase, tier as string.
	if got["userId"] != "auth0|new" {
		t.Errorf("userId: got %v", got["userId"])
	}
	if got["pushEnabled"] != true {
		t.Errorf("pushEnabled: got %v, want true", got["pushEnabled"])
	}
	if got["tier"] != "Free" {
		t.Errorf("tier: got %v, want Free", got["tier"])
	}
	if len(store.saved) != 1 {
		t.Errorf("expected profile persisted once, got %d", len(store.saved))
	}
}

func TestHandler_CreateProfile_Idempotent(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	existing, _ := NewProfile("auth0|abc", "user@example.com", time.Now())
	store.byID["auth0|abc"] = existing
	h := newTestHandler(store, newFakeAuth0(), "")

	rec := httptest.NewRecorder()
	h.create(rec, withSubject(http.MethodPost, "/v1/me", "auth0|abc", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	// Re-creating an existing profile must not register a second one.
	for _, p := range store.saved {
		if p.UserID == "auth0|abc" {
			// allowed only if email backfill occurred; here email already set, so
			// no save should happen.
			t.Errorf("existing profile re-saved unexpectedly")
		}
	}
}

func TestHandler_CreateProfile_AutoGrantsProForVerifiedProDomain(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	h := newTestHandler(store, newFakeAuth0(), "towncrier.test")

	r := withSubject(http.MethodPost, "/v1/me", "auth0|staff", "")
	r.Header.Set("X-Test-Email", "person@towncrier.test")
	r.Header.Set("X-Test-Email-Verified", "true")
	// Emails arrive via JWT claims, threaded through context by the auth layer.
	r = r.WithContext(auth.WithClaims(r.Context(), auth.Claims{
		Subject:       "auth0|staff",
		Email:         "person@towncrier.test",
		EmailVerified: true,
	}))

	rec := httptest.NewRecorder()
	h.create(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["tier"] != "Pro" {
		t.Errorf("tier: got %v, want Pro (auto-grant)", got["tier"])
	}
}

func TestHandler_GetProfile_OkAndNotFound(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	p := &UserProfile{
		UserID:       "auth0|abc",
		Preferences:  NotificationPreferences{PushEnabled: true, DigestDay: time.Wednesday, EmailDigestEnabled: true, SavedDecisionPush: false, SavedDecisionEmail: true},
		Tier:         TierPersonal,
		LastActiveAt: time.Now(),
	}
	store.byID["auth0|abc"] = p
	h := newTestHandler(store, newFakeAuth0(), "")

	rec := httptest.NewRecorder()
	h.get(rec, withSubject(http.MethodGet, "/v1/me", "auth0|abc", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("get status: got %d, want 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// GetUserProfileResult camelCase; DigestDay serialised as the enum NAME.
	if got["userId"] != "auth0|abc" || got["digestDay"] != "Wednesday" || got["tier"] != "Personal" {
		t.Errorf("body wrong: %v", got)
	}
	if got["savedDecisionPush"] != false || got["savedDecisionEmail"] != true {
		t.Errorf("saved-decision flags wrong: %v", got)
	}

	// Missing profile -> bodyless 404 (the error envelope is backfilled by
	// middleware, not the handler).
	rec2 := httptest.NewRecorder()
	h.get(rec2, withSubject(http.MethodGet, "/v1/me", "auth0|missing", ""))
	if rec2.Code != http.StatusNotFound {
		t.Errorf("get missing: got %d, want 404", rec2.Code)
	}
	if rec2.Body.Len() != 0 {
		t.Errorf("404 should be bodyless (middleware backfills), got %q", rec2.Body.String())
	}
}

func TestHandler_PatchProfile_UpdatesAndDefaults(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	existing, _ := NewProfile("auth0|abc", "", time.Now())
	store.byID["auth0|abc"] = existing
	h := newTestHandler(store, newFakeAuth0(), "")

	// iOS-style body: omits digestDay and emailDigestEnabled, which must take the
	// command defaults (Monday / true), exactly as the .NET record defaults do.
	body := `{"pushEnabled":false,"savedDecisionPush":false,"savedDecisionEmail":false}`
	rec := httptest.NewRecorder()
	h.patch(rec, withSubject(http.MethodPatch, "/v1/me", "auth0|abc", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("patch status: got %d, want 200", rec.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["pushEnabled"] != false {
		t.Errorf("pushEnabled: got %v, want false", got["pushEnabled"])
	}
	if got["digestDay"] != "Monday" {
		t.Errorf("digestDay default: got %v, want Monday", got["digestDay"])
	}
	if got["emailDigestEnabled"] != true {
		t.Errorf("emailDigestEnabled default: got %v, want true", got["emailDigestEnabled"])
	}
	if got["savedDecisionPush"] != false {
		t.Errorf("savedDecisionPush: got %v, want false", got["savedDecisionPush"])
	}
}

func TestHandler_PatchProfile_AcceptsDigestDayAsString(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	existing, _ := NewProfile("auth0|abc", "", time.Now())
	store.byID["auth0|abc"] = existing
	h := newTestHandler(store, newFakeAuth0(), "")

	body := `{"pushEnabled":true,"digestDay":"Wednesday","emailDigestEnabled":true,"savedDecisionPush":true,"savedDecisionEmail":true}`
	rec := httptest.NewRecorder()
	h.patch(rec, withSubject(http.MethodPatch, "/v1/me", "auth0|abc", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("patch status: got %d, want 200", rec.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["digestDay"] != "Wednesday" {
		t.Errorf("digestDay: got %v, want Wednesday", got["digestDay"])
	}
}

func TestHandler_PatchProfile_AcceptsDigestDayAsInt(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	existing, _ := NewProfile("auth0|abc", "", time.Now())
	store.byID["auth0|abc"] = existing
	h := newTestHandler(store, newFakeAuth0(), "")

	// System.Text.Json with the string-enum converter still accepts the integer
	// form; the Go handler must too.
	body := `{"pushEnabled":true,"digestDay":3}`
	rec := httptest.NewRecorder()
	h.patch(rec, withSubject(http.MethodPatch, "/v1/me", "auth0|abc", body))

	if rec.Code != http.StatusOK {
		t.Fatalf("patch status: got %d, want 200", rec.Code)
	}
	var got map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["digestDay"] != "Wednesday" {
		t.Errorf("digestDay from int 3: got %v, want Wednesday", got["digestDay"])
	}
}

func TestHandler_PatchProfile_NotFound(t *testing.T) {
	t.Parallel()

	h := newTestHandler(newFakeStore(), newFakeAuth0(), "")
	rec := httptest.NewRecorder()
	h.patch(rec, withSubject(http.MethodPatch, "/v1/me", "auth0|missing", `{"pushEnabled":true}`))
	if rec.Code != http.StatusNotFound {
		t.Errorf("patch missing: got %d, want 404", rec.Code)
	}
}

func TestHandler_DeleteProfile_NoContentAndCascades(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	existing, _ := NewProfile("auth0|abc", "", time.Now())
	store.byID["auth0|abc"] = existing
	a0 := newFakeAuth0()
	h := newTestHandler(store, a0, "")

	rec := httptest.NewRecorder()
	h.delete(rec, withSubject(http.MethodDelete, "/v1/me", "auth0|abc", ""))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status: got %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 should be bodyless, got %q", rec.Body.String())
	}
	if len(store.deleted) != 1 || store.deleted[0] != "auth0|abc" {
		t.Errorf("profile not deleted from store: %v", store.deleted)
	}
	if len(a0.deletedUsers) != 1 || a0.deletedUsers[0] != "auth0|abc" {
		t.Errorf("auth0 user not deleted: %v", a0.deletedUsers)
	}
}

func TestHandler_DeleteProfile_NotFound(t *testing.T) {
	t.Parallel()

	a0 := newFakeAuth0()
	h := newTestHandler(newFakeStore(), a0, "")
	rec := httptest.NewRecorder()
	h.delete(rec, withSubject(http.MethodDelete, "/v1/me", "auth0|missing", ""))
	if rec.Code != http.StatusNotFound {
		t.Errorf("delete missing: got %d, want 404", rec.Code)
	}
	// No Auth0 deletion when the profile did not exist.
	if len(a0.deletedUsers) != 0 {
		t.Errorf("auth0 delete should not run for a missing profile: %v", a0.deletedUsers)
	}
}

func TestHandler_ExportData_NestedContract(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	email := "user@example.com"
	expiry := time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)
	p := &UserProfile{
		UserID:             "auth0|abc",
		Email:              &email,
		Preferences:        NotificationPreferences{PushEnabled: true, DigestDay: time.Friday, EmailDigestEnabled: true, SavedDecisionPush: true, SavedDecisionEmail: false},
		ZonePreferences:    map[string]ZonePreferences{"z1": {NewApplicationPush: true, NewApplicationEmail: false, DecisionPush: true, DecisionEmail: true}},
		Tier:               TierPro,
		SubscriptionExpiry: &expiry,
		LastActiveAt:       time.Now(),
	}
	store.byID["auth0|abc"] = p
	h := newTestHandler(store, newFakeAuth0(), "")

	rec := httptest.NewRecorder()
	h.exportData(rec, withSubject(http.MethodGet, "/v1/me/data", "auth0|abc", ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("export status: got %d, want 200", rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got["userId"] != "auth0|abc" || got["email"] != "user@example.com" {
		t.Errorf("top-level wrong: %v", got)
	}
	np, ok := got["notificationPreferences"].(map[string]any)
	if !ok {
		t.Fatalf("notificationPreferences missing/not object: %v", got["notificationPreferences"])
	}
	if np["digestDay"] != "Friday" || np["savedDecisionEmail"] != false {
		t.Errorf("notificationPreferences wrong: %v", np)
	}
	zps, ok := np["zonePreferences"].([]any)
	if !ok || len(zps) != 1 {
		t.Fatalf("zonePreferences should be a 1-element array: %v", np["zonePreferences"])
	}
	zp := zps[0].(map[string]any)
	if zp["zoneId"] != "z1" || zp["newApplicationEmail"] != false {
		t.Errorf("exported zone prefs wrong: %v", zp)
	}
	sub, ok := got["subscription"].(map[string]any)
	if !ok || sub["tier"] != "Pro" {
		t.Errorf("subscription wrong: %v", got["subscription"])
	}
	// .NET serialises DateTimeOffset with a numeric offset — "+00:00", never
	// Go's RFC 3339 "Z" (caught live by the contract gate on PR #424).
	if sub["expiresAt"] != "2099-12-31T00:00:00+00:00" {
		t.Errorf("expiresAt wire format: got %v, want 2099-12-31T00:00:00+00:00", sub["expiresAt"])
	}
	if sub["gracePeriodExpiresAt"] != nil {
		t.Errorf("gracePeriodExpiresAt: got %v, want null", sub["gracePeriodExpiresAt"])
	}
	// Child collections not yet sourced in this iteration: present as empty arrays.
	for _, k := range []string{"watchZones", "notifications", "savedApplications", "deviceRegistrations", "offerCodeRedemptions"} {
		arr, ok := got[k].([]any)
		if !ok {
			t.Errorf("%s should be an array, got %v", k, got[k])
			continue
		}
		if len(arr) != 0 {
			t.Errorf("%s should be empty in it3, got %v", k, arr)
		}
	}
}

func TestHandler_ExportData_NotFound(t *testing.T) {
	t.Parallel()

	h := newTestHandler(newFakeStore(), newFakeAuth0(), "")
	rec := httptest.NewRecorder()
	h.exportData(rec, withSubject(http.MethodGet, "/v1/me/data", "auth0|missing", ""))
	if rec.Code != http.StatusNotFound {
		t.Errorf("export missing: got %d, want 404", rec.Code)
	}
}
