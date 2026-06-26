package applications

import (
	"testing"
	"time"
)

// TestDecodeDocument covers the raw-document -> domain transform the backfill
// shares with the Cosmos read path: a valid GeoJSON point, an absent point, a
// malformed point, an explicit zero point, and invalid JSON.
func TestDecodeDocument(t *testing.T) {
	t.Parallel()

	const fullDoc = `{
		"id": "24/0001",
		"authorityCode": "100",
		"planitName": "24/0001",
		"uid": "raw-uid-1",
		"areaName": "Testshire",
		"areaId": 100,
		"address": "1 Test Street",
		"postcode": "TE1 1ST",
		"description": "Single storey rear extension",
		"appType": "Full",
		"appState": "Permitted",
		"appSize": "Small",
		"startDate": "2026-01-02",
		"decidedDate": "2026-03-04",
		"consultedDate": "2026-02-03",
		"location": {"type": "Point", "coordinates": [-0.1278, 51.5074]},
		"url": "https://planit.example/24-0001",
		"link": "https://council.example/24-0001",
		"lastDifferent": "2026-06-26T12:00:00+00:00"
	}`

	t.Run("valid point maps all fields and unpacks coordinates", func(t *testing.T) {
		t.Parallel()
		got, err := DecodeDocument([]byte(fullDoc))
		if err != nil {
			t.Fatalf("DecodeDocument: %v", err)
		}
		if got.Name != "24/0001" || got.AreaID != 100 || got.UID != "raw-uid-1" {
			t.Errorf("identity fields: got %+v", got)
		}
		if got.Longitude == nil || got.Latitude == nil {
			t.Fatalf("coordinates: got lon=%v lat=%v, want both set", got.Longitude, got.Latitude)
		}
		if *got.Longitude != -0.1278 || *got.Latitude != 51.5074 {
			t.Errorf("coordinates: got lon=%v lat=%v", *got.Longitude, *got.Latitude)
		}
		if got.AppState == nil || *got.AppState != "Permitted" {
			t.Errorf("appState: got %v", got.AppState)
		}
		if got.StartDate == nil || !got.StartDate.Equal(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("startDate: got %v", got.StartDate)
		}
		if !got.LastDifferent.Equal(time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)) {
			t.Errorf("lastDifferent: got %v", got.LastDifferent)
		}
	})

	t.Run("absent location yields nil coordinates without error", func(t *testing.T) {
		t.Parallel()
		const noLoc = `{"planitName":"24/0002","authorityCode":"100","areaId":100,"uid":"u2","areaName":"Testshire","address":"a","description":"d","lastDifferent":"2026-06-26T12:00:00+00:00"}`
		got, err := DecodeDocument([]byte(noLoc))
		if err != nil {
			t.Fatalf("DecodeDocument: %v", err)
		}
		if got.Longitude != nil || got.Latitude != nil {
			t.Errorf("coordinates: got lon=%v lat=%v, want nil/nil", got.Longitude, got.Latitude)
		}
		if got.Name != "24/0002" {
			t.Errorf("name: got %q", got.Name)
		}
	})

	t.Run("malformed point with one coordinate yields nil coordinates", func(t *testing.T) {
		t.Parallel()
		const oneCoord = `{"planitName":"24/0003","authorityCode":"100","areaId":100,"uid":"u3","areaName":"Testshire","address":"a","description":"d","location":{"type":"Point","coordinates":[-0.1278]},"lastDifferent":"2026-06-26T12:00:00+00:00"}`
		got, err := DecodeDocument([]byte(oneCoord))
		if err != nil {
			t.Fatalf("DecodeDocument: %v", err)
		}
		if got.Longitude != nil || got.Latitude != nil {
			t.Errorf("coordinates: got lon=%v lat=%v, want nil/nil", got.Longitude, got.Latitude)
		}
	})

	t.Run("explicit zero point is preserved not treated as missing", func(t *testing.T) {
		t.Parallel()
		const zeroPoint = `{"planitName":"24/0004","authorityCode":"100","areaId":100,"uid":"u4","areaName":"Testshire","address":"a","description":"d","location":{"type":"Point","coordinates":[0,0]},"lastDifferent":"2026-06-26T12:00:00+00:00"}`
		got, err := DecodeDocument([]byte(zeroPoint))
		if err != nil {
			t.Fatalf("DecodeDocument: %v", err)
		}
		if got.Longitude == nil || got.Latitude == nil {
			t.Fatalf("coordinates: got lon=%v lat=%v, want both set to zero", got.Longitude, got.Latitude)
		}
		if *got.Longitude != 0 || *got.Latitude != 0 {
			t.Errorf("coordinates: got lon=%v lat=%v, want 0/0", *got.Longitude, *got.Latitude)
		}
	})

	t.Run("invalid json is an error", func(t *testing.T) {
		t.Parallel()
		if _, err := DecodeDocument([]byte("not json")); err == nil {
			t.Fatal("DecodeDocument: want error for invalid json, got nil")
		}
	})
}
