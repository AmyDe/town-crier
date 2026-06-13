package applications

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApplicationDocument_RoundTrip(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	got := newApplicationDocument(a).toDomain()

	if got.Name != a.Name || got.UID != a.UID || got.AreaName != a.AreaName || got.AreaID != a.AreaID {
		t.Errorf("identity mismatch: got %+v", got)
	}
	if got.Address != a.Address || *got.Postcode != *a.Postcode || got.Description != a.Description {
		t.Errorf("address fields mismatch: got %+v", got)
	}
	if *got.AppType != *a.AppType || *got.AppState != *a.AppState || *got.AppSize != *a.AppSize {
		t.Errorf("app fields mismatch: got %+v", got)
	}
	if !got.StartDate.Equal(*a.StartDate) || !got.DecidedDate.Equal(*a.DecidedDate) || !got.ConsultedDate.Equal(*a.ConsultedDate) {
		t.Errorf("dates mismatch: got start=%v decided=%v consulted=%v", got.StartDate, got.DecidedDate, got.ConsultedDate)
	}
	if *got.Longitude != *a.Longitude || *got.Latitude != *a.Latitude {
		t.Errorf("coords mismatch: got lon=%v lat=%v", *got.Longitude, *got.Latitude)
	}
	if !got.LastDifferent.Equal(a.LastDifferent) {
		t.Errorf("lastDifferent: got %v, want %v", got.LastDifferent, a.LastDifferent)
	}
}

func TestApplicationDocument_CamelCaseGeoJSONAndDates(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(newApplicationDocument(testApplication(t)))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for _, key := range []string{
		`"id":`, `"authorityCode":"471"`, `"planitName":`, `"uid":`, `"areaName":`, `"areaId":471`,
		`"address":`, `"postcode":`, `"description":`, `"appType":`, `"appState":`, `"appSize":`,
		`"startDate":"2026-01-05"`, `"decidedDate":"2026-03-01"`, `"consultedDate":"2026-01-20"`,
		`"location":`, `"url":`, `"link":`, `"lastDifferent":"2026-03-02T09:30:00+00:00"`,
	} {
		if !strings.Contains(body, key) {
			t.Errorf("missing %s in %s", key, body)
		}
	}
	// GeoJSON point with [lon, lat] order and a "Point" type.
	if !strings.Contains(body, `"type":"Point"`) || !strings.Contains(body, `"coordinates":[-0.0931,51.5155]`) {
		t.Errorf("geojson point wrong: %s", body)
	}
}

func TestApplicationDocument_AbsentCoordsAndDatesAreNull(t *testing.T) {
	t.Parallel()
	a := PlanningApplication{Name: "n", UID: "u", AreaName: "a", AreaID: 1, Address: "x", Description: "d"}
	raw, err := json.Marshal(newApplicationDocument(a))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for _, key := range []string{`"location":null`, `"startDate":null`, `"postcode":null`, `"appType":null`} {
		if !strings.Contains(body, key) {
			t.Errorf("expected %s in %s", key, body)
		}
	}
	// And a round-trip leaves coordinates absent.
	got := newApplicationDocument(a).toDomain()
	if got.Longitude != nil || got.Latitude != nil || got.StartDate != nil {
		t.Errorf("absent fields hydrated non-nil: %+v", got)
	}
}
