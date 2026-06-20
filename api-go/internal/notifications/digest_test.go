package notifications

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDigestNotificationDocument_RoundTrips(t *testing.T) {
	t.Parallel()
	// A full Notifications-container document hydrates into a DigestNotification
	// carrying every field the digest email body and APNs payload need. The
	// camelCase JSON keys match the Cosmos document contract.
	raw := []byte(`{
		"id": "n-1",
		"userId": "user-1",
		"applicationUid": "19/00123/FUL",
		"applicationName": "19/00123/FUL",
		"watchZoneId": "zone-1",
		"applicationAddress": "10 High St",
		"applicationDescription": "Single storey rear extension",
		"applicationType": "Householder",
		"authorityId": 42,
		"decision": "Permitted",
		"eventType": "DecisionUpdate",
		"sources": "Zone, Saved",
		"pushSent": true,
		"emailSent": false,
		"createdAt": "2026-02-01T10:00:00+00:00"
	}`)

	var doc digestDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	n := doc.toDigest()

	if n.ID != "n-1" || n.UserID != "user-1" {
		t.Errorf("ids: got id=%q userId=%q", n.ID, n.UserID)
	}
	if n.ApplicationUID != "19/00123/FUL" || n.ApplicationName != "19/00123/FUL" {
		t.Errorf("application uid/name: got %q / %q", n.ApplicationUID, n.ApplicationName)
	}
	if n.WatchZoneID == nil || *n.WatchZoneID != "zone-1" {
		t.Errorf("watchZoneId: got %v", n.WatchZoneID)
	}
	if n.ApplicationAddress != "10 High St" {
		t.Errorf("address: got %q", n.ApplicationAddress)
	}
	if n.ApplicationDescription != "Single storey rear extension" {
		t.Errorf("description: got %q", n.ApplicationDescription)
	}
	if n.ApplicationType == nil || *n.ApplicationType != "Householder" {
		t.Errorf("type: got %v", n.ApplicationType)
	}
	if n.Decision == nil || *n.Decision != "Permitted" {
		t.Errorf("decision: got %v", n.Decision)
	}
	if n.EventType != EventDecisionUpdate {
		t.Errorf("eventType: got %q", n.EventType)
	}
	if !n.HasSavedSource() {
		t.Error("sources should include Saved")
	}
	wantCreated := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	if !n.CreatedAt.Equal(wantCreated) {
		t.Errorf("createdAt: got %v, want %v", n.CreatedAt, wantCreated)
	}
}

func TestDigestNotificationDocument_LegacyNullsCoalesce(t *testing.T) {
	t.Parallel()
	// Legacy rows predating eventType/sources/applicationUid hydrate with the
	// backfill defaults: NewApplication event, no Saved source, applicationUid
	// falls back to applicationName.
	raw := []byte(`{
		"id": "n-2",
		"userId": "user-2",
		"applicationName": "20/00045/FUL",
		"applicationAddress": "5 Mill Ln",
		"applicationDescription": "New dwelling",
		"authorityId": 7,
		"pushSent": false,
		"createdAt": "2026-01-01T00:00:00+00:00"
	}`)

	var doc digestDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	n := doc.toDigest()

	if n.EventType != EventNewApplication {
		t.Errorf("legacy eventType: got %q, want NewApplication", n.EventType)
	}
	if n.HasSavedSource() {
		t.Error("legacy row should not have Saved source")
	}
	if n.ApplicationUID != "20/00045/FUL" {
		t.Errorf("legacy applicationUid fallback: got %q", n.ApplicationUID)
	}
	if n.WatchZoneID != nil {
		t.Errorf("absent watchZoneId should be nil, got %v", n.WatchZoneID)
	}
	if n.ApplicationType != nil {
		t.Errorf("absent applicationType should be nil, got %v", n.ApplicationType)
	}
}

func TestDigestNotification_MarkEmailSentRoundTripsDocument(t *testing.T) {
	t.Parallel()
	// MarkEmailSent flips emailSent so the persisted document excludes the row
	// from the next hourly cycle's GetUnsentEmailsByUser query.
	n := DigestNotification{
		ID:                     "n-3",
		UserID:                 "user-3",
		ApplicationUID:         "21/0001",
		ApplicationName:        "21/0001",
		ApplicationAddress:     "1 Test Rd",
		ApplicationDescription: "desc",
		AuthorityID:            1,
		EventType:              EventNewApplication,
		CreatedAt:              time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		EmailSent:              false,
	}
	n.MarkEmailSent()
	if !n.EmailSent {
		t.Fatal("MarkEmailSent should set EmailSent true")
	}

	body, err := json.Marshal(newDigestDocument(n))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back digestDocument
	if err := json.Unmarshal(body, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.toDigest().EmailSent != true {
		t.Error("emailSent should survive the document round trip")
	}
	// The 90-day TTL must be written so digest rows expire on schedule.
	if back.TTL <= 0 {
		t.Errorf("ttl should be a positive value, got %d", back.TTL)
	}
}
