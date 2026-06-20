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
