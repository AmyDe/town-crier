package profiles

import (
	"testing"
)

// TestDecodeDocument_Valid hydrates a well-formed Cosmos Users-container
// document into a domain UserProfile, exercising the field-mapping that the
// backfill shares with the read path.
func TestDecodeDocument_Valid(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"id": "auth0|u1",
		"userId": "auth0|u1",
		"email": "alice@example.com",
		"pushEnabled": true,
		"digestDay": 1,
		"emailDigestEnabled": false,
		"savedDecisionPush": true,
		"savedDecisionEmail": false,
		"zonePreferences": {
			"zone-1": {"newApplicationPush": true, "newApplicationEmail": false, "decisionPush": true, "decisionEmail": false}
		},
		"tier": "Free",
		"lastActiveAt": "2026-06-26T12:00:00Z",
		"lastActiveAtEpoch": 1719403200000
	}`)

	p, err := DecodeDocument(raw)
	if err != nil {
		t.Fatalf("DecodeDocument: %v", err)
	}
	if p.UserID != "auth0|u1" {
		t.Errorf("UserID = %q, want %q", p.UserID, "auth0|u1")
	}
	if p.Email == nil || *p.Email != "alice@example.com" {
		t.Errorf("Email = %v, want alice@example.com", p.Email)
	}
	if p.Preferences.PushEnabled != true {
		t.Errorf("PushEnabled = %v, want true", p.Preferences.PushEnabled)
	}
	if p.Preferences.EmailDigestEnabled != false {
		t.Errorf("EmailDigestEnabled = %v, want false (explicitly false, not absent)", p.Preferences.EmailDigestEnabled)
	}
	if p.Tier != TierFree {
		t.Errorf("Tier = %v, want TierFree", p.Tier)
	}
	if len(p.ZonePreferences) != 1 {
		t.Errorf("ZonePreferences len = %d, want 1", len(p.ZonePreferences))
	}
}

// TestDecodeDocument_CoalesceTrue verifies the opt-in default: a document
// written before emailDigestEnabled existed (absent / null) hydrates as true
// rather than the Go zero-value false (coalesceTrue, bead tc-hpd2.8).
func TestDecodeDocument_CoalesceTrue(t *testing.T) {
	t.Parallel()
	// No emailDigestEnabled / savedDecision* fields → must coalesce to true.
	raw := []byte(`{
		"id": "auth0|u2",
		"userId": "auth0|u2",
		"pushEnabled": false,
		"digestDay": 0,
		"zonePreferences": {},
		"tier": "Free",
		"lastActiveAt": "2026-06-26T12:00:00Z",
		"lastActiveAtEpoch": 1719403200000
	}`)

	p, err := DecodeDocument(raw)
	if err != nil {
		t.Fatalf("DecodeDocument: %v", err)
	}
	if !p.Preferences.EmailDigestEnabled {
		t.Error("EmailDigestEnabled should coalesce to true when absent")
	}
	if !p.Preferences.SavedDecisionPush {
		t.Error("SavedDecisionPush should coalesce to true when absent")
	}
	if !p.Preferences.SavedDecisionEmail {
		t.Error("SavedDecisionEmail should coalesce to true when absent")
	}
}

// TestDecodeDocument_InvalidJSON confirms that malformed JSON returns an error.
func TestDecodeDocument_InvalidJSON(t *testing.T) {
	t.Parallel()
	if _, err := DecodeDocument([]byte("not json")); err == nil {
		t.Fatal("DecodeDocument: want error for invalid JSON, got nil")
	}
}

// TestDecodeDocument_UnknownTier confirms that an unrecognised tier string
// returns an error rather than silently defaulting.
func TestDecodeDocument_UnknownTier(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"id": "auth0|u3",
		"userId": "auth0|u3",
		"pushEnabled": false,
		"digestDay": 0,
		"zonePreferences": {},
		"tier": "Platinum",
		"lastActiveAt": "2026-06-26T12:00:00Z",
		"lastActiveAtEpoch": 1719403200000
	}`)
	if _, err := DecodeDocument(raw); err == nil {
		t.Fatal("DecodeDocument: want error for unknown tier, got nil")
	}
}
