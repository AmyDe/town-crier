package profiles

import (
	"encoding/json"
	"testing"
	"time"
)

func TestProfileDocument_JSONShapeMatchesNET(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 11, 9, 30, 0, 0, time.UTC)
	expiry := time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)
	email := "user@example.com"
	p := &UserProfile{
		UserID: "auth0|abc",
		Email:  &email,
		Preferences: NotificationPreferences{
			PushEnabled:        true,
			DigestDay:          time.Wednesday,
			EmailDigestEnabled: false,
			SavedDecisionPush:  true,
			SavedDecisionEmail: false,
		},
		ZonePreferences: map[string]ZonePreferences{
			"zone-1": {NewApplicationPush: true, NewApplicationEmail: false, DecisionPush: true, DecisionEmail: true},
		},
		Tier:               TierPro,
		SubscriptionExpiry: &expiry,
		LastActiveAt:       now,
	}

	b, err := json.Marshal(newProfileDocument(p))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// camelCase keys matching the stored document shape.
	wantStr := map[string]string{
		"id":     "auth0|abc",
		"userId": "auth0|abc",
		"email":  "user@example.com",
		"tier":   "Pro",
	}
	for k, want := range wantStr {
		if got, _ := doc[k].(string); got != want {
			t.Errorf("doc[%q]: got %v, want %q", k, doc[k], want)
		}
	}

	// digestDay serialises as an integer (no string-enum converter on the Cosmos
	// document); Wednesday == 3 in Go's time.Weekday.
	if got, _ := doc["digestDay"].(float64); got != 3 {
		t.Errorf("doc[digestDay]: got %v, want 3 (Wednesday)", doc["digestDay"])
	}
	if doc["pushEnabled"] != true {
		t.Errorf("doc[pushEnabled]: got %v, want true", doc["pushEnabled"])
	}
	if doc["emailDigestEnabled"] != false {
		t.Errorf("doc[emailDigestEnabled]: got %v, want false", doc["emailDigestEnabled"])
	}

	zone, ok := doc["zonePreferences"].(map[string]any)
	if !ok {
		t.Fatalf("zonePreferences not an object: %v", doc["zonePreferences"])
	}
	z1, ok := zone["zone-1"].(map[string]any)
	if !ok {
		t.Fatalf("zone-1 missing: %v", zone)
	}
	if z1["newApplicationEmail"] != false || z1["decisionPush"] != true {
		t.Errorf("zone-1 prefs wrong casing/values: %v", z1)
	}
}

func TestProfileDocument_RoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 11, 9, 30, 0, 0, time.UTC)
	email := "user@example.com"
	p := &UserProfile{
		UserID:          "auth0|abc",
		Email:           &email,
		Preferences:     DefaultPreferences(),
		ZonePreferences: map[string]ZonePreferences{"z": {NewApplicationPush: true}},
		Tier:            TierPersonal,
		LastActiveAt:    now,
	}

	b, err := json.Marshal(newProfileDocument(p))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var doc profileDocument
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, err := doc.toDomain()
	if err != nil {
		t.Fatalf("toDomain: %v", err)
	}

	if got.UserID != p.UserID || got.Tier != p.Tier || !got.LastActiveAt.Equal(now) {
		t.Errorf("round-trip mismatch: %+v vs %+v", got, p)
	}
	if got.Email == nil || *got.Email != email {
		t.Errorf("email round-trip: got %v", got.Email)
	}
	if got.ZonePreferences["z"].NewApplicationPush != true {
		t.Errorf("zone prefs round-trip lost data: %v", got.ZonePreferences)
	}
}

func TestProfileDocument_LegacyDefaultsTrue(t *testing.T) {
	t.Parallel()

	// Legacy documents predating emailDigestEnabled / savedDecision* hydrate as
	// opt-in (true) via coalesceTrue. Missing fields must not become false.
	raw := `{"id":"u1","userId":"u1","pushEnabled":true,"digestDay":1,"tier":"Free","lastActiveAt":"2026-06-11T09:30:00Z"}`
	var doc profileDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, err := doc.toDomain()
	if err != nil {
		t.Fatalf("toDomain: %v", err)
	}
	if !got.Preferences.EmailDigestEnabled || !got.Preferences.SavedDecisionPush || !got.Preferences.SavedDecisionEmail {
		t.Errorf("legacy doc should hydrate email/saved-decision flags as true: %+v", got.Preferences)
	}
	if got.ZonePreferences == nil {
		t.Error("zonePreferences should hydrate as non-nil empty map")
	}
}

func TestProfileDocument_OmitsNilOptionals(t *testing.T) {
	t.Parallel()

	// A free profile with no subscription/email leaves the optional fields as
	// JSON null (the fields are present with null values, not omitted).
	p, _ := NewProfile("u1", "", time.Now())
	b, err := json.Marshal(newProfileDocument(p))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"email", "subscriptionExpiry", "originalTransactionId", "gracePeriodExpiry"} {
		v, present := doc[k]
		if !present {
			t.Errorf("field %q should be present (as null), not omitted", k)
		}
		if v != nil {
			t.Errorf("field %q should be null for a fresh free profile, got %v", k, v)
		}
	}
}
