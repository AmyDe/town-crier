package applications

import (
	"testing"
	"time"
)

// testApplication is a fully-populated planning application used across the
// package's tests.
func testApplication(t *testing.T) PlanningApplication {
	t.Helper()
	postcode := "EC2V 5AE"
	appType := "Full"
	appState := "Permitted"
	appSize := "Small"
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	decided := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	consulted := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
	lon := -0.0931
	lat := 51.5155
	url := "https://planit.example/app"
	link := "https://council.example/app"
	return PlanningApplication{
		Name:          "24/0123/FUL",
		UID:           "ABC-24-0123",
		AreaName:      "City of London",
		AreaID:        471,
		Address:       "1 Test Street",
		Postcode:      &postcode,
		Description:   "Single storey extension",
		AppType:       &appType,
		AppState:      &appState,
		AppSize:       &appSize,
		StartDate:     &start,
		DecidedDate:   &decided,
		ConsultedDate: &consulted,
		Longitude:     &lon,
		Latitude:      &lat,
		URL:           &url,
		Link:          &link,
		LastDifferent: time.Date(2026, 3, 2, 9, 30, 0, 0, time.UTC),
	}
}

func TestPlanningApplication_CanonicalUID(t *testing.T) {
	t.Parallel()
	a := PlanningApplication{Name: "24/0123/FUL", AreaID: 471}
	if got := a.CanonicalUID(); got != "471/24/0123/FUL" {
		t.Errorf("CanonicalUID: got %q, want 471/24/0123/FUL", got)
	}
}

func TestPlanningApplication_CanonicalUID_PreservesNameWithSlashes(t *testing.T) {
	t.Parallel()
	a := PlanningApplication{Name: "P/2026/0044/HH", AreaID: 9}
	if got := a.CanonicalUID(); got != "9/P/2026/0044/HH" {
		t.Errorf("CanonicalUID: got %q", got)
	}
}
