package savedapplications

import (
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
