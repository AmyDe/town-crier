package savedapplications

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

func testApp(t *testing.T) applications.PlanningApplication {
	t.Helper()
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	lon := -0.0931
	lat := 51.5155
	return applications.PlanningApplication{
		Name:          "24/0123/FUL",
		UID:           "ABC-24-0123",
		AreaName:      "City of London",
		AreaID:        471,
		Address:       "1 Test Street",
		Description:   "Extension",
		StartDate:     &start,
		Longitude:     &lon,
		Latitude:      &lat,
		LastDifferent: time.Date(2026, 3, 2, 9, 30, 0, 0, time.UTC),
	}
}

func TestNewSavedApplication_CanonicalKeyAndSnapshot(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)
	s := NewSavedApplication("auth0|u", testApp(t), now)

	if s.ApplicationUID != "471/24/0123/FUL" {
		t.Errorf("uid: got %q, want canonical 471/24/0123/FUL", s.ApplicationUID)
	}
	if s.AuthorityID != 471 {
		t.Errorf("authorityId: got %d", s.AuthorityID)
	}
	if s.Application == nil || s.Application.Name != "24/0123/FUL" {
		t.Errorf("snapshot not embedded: %+v", s.Application)
	}
	if !s.SavedAt.Equal(now) {
		t.Errorf("savedAt: got %v", s.SavedAt)
	}
}

func TestSavedApplicationDocument_RoundTrip(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)
	s := NewSavedApplication("auth0|u", testApp(t), now)
	got := newSavedApplicationDocument(s).toDomain()

	if got.UserID != s.UserID || got.ApplicationUID != s.ApplicationUID || got.AuthorityID != s.AuthorityID {
		t.Errorf("identity mismatch: %+v", got)
	}
	if !got.SavedAt.Equal(s.SavedAt) {
		t.Errorf("savedAt: got %v", got.SavedAt)
	}
	if got.Application == nil || got.Application.UID != s.Application.UID {
		t.Errorf("snapshot lost: %+v", got.Application)
	}
}

func TestSavedApplicationDocument_CamelCaseIdAndSavedAt(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 13, 8, 0, 0, 0, time.UTC)
	raw, err := json.Marshal(newSavedApplicationDocument(NewSavedApplication("auth0|u", testApp(t), now)))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for _, key := range []string{
		`"id":"auth0|u:471/24/0123/FUL"`, `"userId":"auth0|u"`, `"applicationUid":"471/24/0123/FUL"`,
		`"authorityId":471`, `"savedAt":"2026-06-13T08:00:00+00:00"`, `"application":`,
	} {
		if !strings.Contains(body, key) {
			t.Errorf("missing %s in %s", key, body)
		}
	}
}

func TestSavedApplicationDocument_AuthorityIdCoalescesFromSnapshot(t *testing.T) {
	t.Parallel()
	// Legacy row: authorityId column absent, but the embedded snapshot carries areaId.
	raw := `{"id":"u:1/x","userId":"u","applicationUid":"1/x","savedAt":"2026-06-13T08:00:00+00:00","application":{"name":"x","uid":"u1","areaName":"a","areaId":42,"address":"addr","description":"d","lastDifferent":"2026-06-13T08:00:00+00:00"}}`
	var doc savedApplicationDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := doc.toDomain()
	if got.AuthorityID != 42 {
		t.Errorf("authorityId should coalesce from snapshot areaId: got %d", got.AuthorityID)
	}
}
