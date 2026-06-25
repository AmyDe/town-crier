package watchzones

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestWatchZoneDocument_RoundTrip(t *testing.T) {
	t.Parallel()
	z := testZone(t)

	got, err := newWatchZoneDocument(z).toDomain()
	if err != nil {
		t.Fatalf("toDomain: %v", err)
	}
	if got.ID != z.ID || got.UserID != z.UserID || got.Name != z.Name {
		t.Errorf("identity mismatch: got %+v", got)
	}
	if got.Latitude != z.Latitude || got.Longitude != z.Longitude || got.RadiusMetres != z.RadiusMetres {
		t.Errorf("geometry mismatch: got %+v", got)
	}
	if got.AuthorityID != z.AuthorityID {
		t.Errorf("authorityId: got %d, want %d", got.AuthorityID, z.AuthorityID)
	}
	if got.PushEnabled != z.PushEnabled || got.EmailInstantEnabled != z.EmailInstantEnabled {
		t.Errorf("flags mismatch: got %+v", got)
	}
	if !got.CreatedAt.Equal(z.CreatedAt) {
		t.Errorf("createdAt: got %v, want %v", got.CreatedAt, z.CreatedAt)
	}
}

func TestWatchZoneDocument_CamelCaseAndDotNetTime(t *testing.T) {
	t.Parallel()
	z := testZone(t)

	raw, err := json.Marshal(newWatchZoneDocument(z))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)

	for _, key := range []string{
		`"id":`, `"userId":`, `"name":`, `"latitude":`, `"longitude":`,
		`"radiusMetres":`, `"authorityId":`, `"createdAt":`, `"pushEnabled":`, `"emailInstantEnabled":`,
	} {
		if !strings.Contains(body, key) {
			t.Errorf("missing camelCase key %s in %s", key, body)
		}
	}
	// createdAt must serialise with a numeric UTC offset ("+00:00"), never RFC 3339 Z.
	if !strings.Contains(body, "2026-06-01T09:00:00+00:00") {
		t.Errorf("createdAt not in numeric-offset format: %s", body)
	}
	if strings.Contains(body, "09:00:00Z") {
		t.Errorf("createdAt used RFC 3339 Z suffix: %s", body)
	}
}

func TestWatchZoneDocument_WritesGeoJSONLocation(t *testing.T) {
	t.Parallel()
	z := testZone(t) // latitude 51.5074, longitude -0.1278

	raw, err := json.Marshal(newWatchZoneDocument(z))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)

	if !strings.Contains(body, `"location":`) {
		t.Errorf("missing location key in %s", body)
	}
	if !strings.Contains(body, `"type":"Point"`) {
		t.Errorf("location is not a GeoJSON Point in %s", body)
	}
	// GeoJSON coordinates are [longitude, latitude] — order matters and must
	// match what a spatial index on /location expects (ST_DISTANCE).
	if !strings.Contains(body, `"coordinates":[-0.1278,51.5074]`) {
		t.Errorf("location coordinates not [lon,lat]: %s", body)
	}
}

func TestWatchZoneDocument_WritesBoundingBox(t *testing.T) {
	t.Parallel()
	z := testZone(t)

	raw, err := json.Marshal(newWatchZoneDocument(z))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)

	for _, key := range []string{`"minLat":`, `"maxLat":`, `"minLon":`, `"maxLon":`} {
		if !strings.Contains(body, key) {
			t.Errorf("missing bounding-box key %s in %s", key, body)
		}
	}

	// The persisted box must equal the domain-computed box: a freshly written zone
	// always carries the bbox so the notify-path prune can use it without a backfill.
	wantMinLat, wantMaxLat, wantMinLon, wantMaxLon := z.boundingBox()
	var doc watchZoneDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, c := range []struct {
		name string
		got  *float64
		want float64
	}{
		{"minLat", doc.MinLat, wantMinLat},
		{"maxLat", doc.MaxLat, wantMaxLat},
		{"minLon", doc.MinLon, wantMinLon},
		{"maxLon", doc.MaxLon, wantMaxLon},
	} {
		if c.got == nil {
			t.Errorf("%s must be written, got nil", c.name)
			continue
		}
		if *c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, *c.got, c.want)
		}
	}
}

func TestWatchZoneDocument_LegacyMissingBoundingBoxDecodesNil(t *testing.T) {
	t.Parallel()
	// A document written before the bounding box existed omits minLat/maxLat/
	// minLon/maxLon. They are *float64 so they decode to nil rather than 0 — that
	// nil is the "needs backfill" signal the slice-3 CLI backfill keys on, and the
	// transitional query's NOT IS_DEFINED(c.minLat) fallback matches such zones via
	// the ST_DISTANCE residual until the backfill runs.
	raw := `{"id":"z1","userId":"u1","name":"Home","latitude":51.5074,"longitude":-0.1278,"radiusMetres":1000,"authorityId":471,"createdAt":"2026-06-01T09:00:00+00:00"}`
	var doc watchZoneDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.MinLat != nil || doc.MaxLat != nil || doc.MinLon != nil || doc.MaxLon != nil {
		t.Errorf("legacy doc must decode the bounding box as nil: %+v %+v %+v %+v",
			doc.MinLat, doc.MaxLat, doc.MinLon, doc.MaxLon)
	}
	// Hydration must still succeed — the bbox is not read by toDomain (the domain
	// recomputes it from centre + radius).
	if _, err := doc.toDomain(); err != nil {
		t.Fatalf("toDomain on a legacy bbox-less doc: %v", err)
	}
}

func TestWatchZoneDocument_HydratesWithAndWithoutLocation(t *testing.T) {
	t.Parallel()
	// location is additive: hydration reads the latitude/longitude float columns,
	// so a legacy document predating location and a new document carrying it both
	// hydrate to the same coordinates. Adding the field changes no read behaviour.
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "without location (legacy)",
			raw:  `{"id":"z1","userId":"u1","name":"Home","latitude":51.5074,"longitude":-0.1278,"radiusMetres":1000,"authorityId":471,"createdAt":"2026-06-01T09:00:00+00:00"}`,
		},
		{
			name: "with location",
			raw:  `{"id":"z1","userId":"u1","name":"Home","latitude":51.5074,"longitude":-0.1278,"radiusMetres":1000,"authorityId":471,"createdAt":"2026-06-01T09:00:00+00:00","location":{"type":"Point","coordinates":[-0.1278,51.5074]}}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var doc watchZoneDocument
			if err := json.Unmarshal([]byte(tc.raw), &doc); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got, err := doc.toDomain()
			if err != nil {
				t.Fatalf("toDomain: %v", err)
			}
			if got.Latitude != 51.5074 || got.Longitude != -0.1278 {
				t.Errorf("coords mismatch: got lat=%v lon=%v", got.Latitude, got.Longitude)
			}
		})
	}
}

func TestWatchZoneDocument_LegacyNullFlagsCoalesceTrue(t *testing.T) {
	t.Parallel()
	// A document written before the per-zone flags existed omits them; absent
	// bool fields must coalesce to true (the opt-in default).
	raw := `{"id":"z1","userId":"u1","name":"Home","latitude":51,"longitude":-0.1,"radiusMetres":500,"authorityId":471,"createdAt":"2026-06-01T09:00:00+00:00"}`
	var doc watchZoneDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, err := doc.toDomain()
	if err != nil {
		t.Fatalf("toDomain: %v", err)
	}
	if !got.PushEnabled || !got.EmailInstantEnabled {
		t.Errorf("absent flags must coalesce to true: push=%v email=%v", got.PushEnabled, got.EmailInstantEnabled)
	}
}

func TestWatchZoneDocument_AbsentCreatedAtHydratesToZero(t *testing.T) {
	t.Parallel()
	raw := `{"id":"z1","userId":"u1","name":"Home","latitude":51,"longitude":-0.1,"radiusMetres":500,"authorityId":471}`
	var doc watchZoneDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, err := doc.toDomain()
	if err != nil {
		t.Fatalf("toDomain: %v", err)
	}
	if !got.CreatedAt.Equal(time.Time{}) {
		t.Errorf("absent createdAt should hydrate to zero time, got %v", got.CreatedAt)
	}
}
