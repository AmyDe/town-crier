package watchzones

import (
	"testing"
	"time"
)

// TestDecodeDocument covers the raw-document -> domain transform the watch-zone
// backfill (cmd/pgbackfill-zones) shares with the Cosmos read path: a full
// document, a legacy-minimal document whose nullable fields coalesce, invalid
// JSON, and a document that violates a domain invariant.
func TestDecodeDocument(t *testing.T) {
	t.Parallel()

	const fullDoc = `{
		"id": "11111111-1111-1111-1111-111111111111",
		"userId": "auth0|user-1",
		"name": "Home",
		"latitude": 51.5074,
		"longitude": -0.1278,
		"radiusMetres": 1000,
		"authorityId": 100,
		"location": {"type": "Point", "coordinates": [-0.1278, 51.5074]},
		"minLat": 51.49,
		"maxLat": 51.52,
		"minLon": -0.14,
		"maxLon": -0.11,
		"createdAt": "2026-06-26T12:00:00+00:00",
		"pushEnabled": false,
		"emailInstantEnabled": false
	}`

	t.Run("valid document maps all fields", func(t *testing.T) {
		t.Parallel()
		got, err := DecodeDocument([]byte(fullDoc))
		if err != nil {
			t.Fatalf("DecodeDocument: %v", err)
		}
		if got.ID != "11111111-1111-1111-1111-111111111111" || got.UserID != "auth0|user-1" || got.Name != "Home" {
			t.Errorf("identity fields: got %+v", got)
		}
		if got.Latitude != 51.5074 || got.Longitude != -0.1278 {
			t.Errorf("coordinates: got lat=%v lon=%v", got.Latitude, got.Longitude)
		}
		if got.RadiusMetres != 1000 || got.AuthorityID != 100 {
			t.Errorf("radius/authority: got radius=%v authority=%v", got.RadiusMetres, got.AuthorityID)
		}
		if got.PushEnabled || got.EmailInstantEnabled {
			t.Errorf("flags: got push=%v email=%v, want both false (explicit)", got.PushEnabled, got.EmailInstantEnabled)
		}
		if !got.CreatedAt.Equal(time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)) {
			t.Errorf("createdAt: got %v", got.CreatedAt)
		}
	})

	t.Run("absent flags coalesce to true and absent createdAt is the zero instant", func(t *testing.T) {
		t.Parallel()
		const minimal = `{"id":"22222222-2222-2222-2222-222222222222","userId":"u2","name":"Work","latitude":51.5,"longitude":-0.1,"radiusMetres":500,"authorityId":100}`
		got, err := DecodeDocument([]byte(minimal))
		if err != nil {
			t.Fatalf("DecodeDocument: %v", err)
		}
		if !got.PushEnabled || !got.EmailInstantEnabled {
			t.Errorf("flags: got push=%v email=%v, want both true (legacy opt-in default)", got.PushEnabled, got.EmailInstantEnabled)
		}
		if !got.CreatedAt.IsZero() {
			t.Errorf("createdAt: got %v, want zero instant", got.CreatedAt)
		}
	})

	t.Run("invalid json is an error", func(t *testing.T) {
		t.Parallel()
		if _, err := DecodeDocument([]byte("not json")); err == nil {
			t.Fatal("DecodeDocument: want error for invalid json, got nil")
		}
	})

	t.Run("document violating a domain invariant is rejected", func(t *testing.T) {
		t.Parallel()
		// radiusMetres <= 0 violates the NewWatchZone constructor.
		const badRadius = `{"id":"33333333-3333-3333-3333-333333333333","userId":"u3","name":"Bad","latitude":51.5,"longitude":-0.1,"radiusMetres":0,"authorityId":100}`
		if _, err := DecodeDocument([]byte(badRadius)); err == nil {
			t.Fatal("DecodeDocument: want error for non-positive radius, got nil")
		}
	})
}
