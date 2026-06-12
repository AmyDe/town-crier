package devicetokens

import (
	"encoding/json"
	"testing"
	"time"
)

// TestDeviceDocument_WireShape pins the exact Cosmos document the .NET
// CosmosDeviceRegistrationRepository writes: id == token, partition field
// userId, token, the string platform, the +00:00 registeredAt, and the 180-day
// TTL. A drift here would make a Go-written document incompatible with the
// existing container and break the GDPR export contract.
func TestDeviceDocument_WireShape(t *testing.T) {
	t.Parallel()

	reg := DeviceRegistration{
		UserID:       "auth0|u1",
		Token:        "tok-abc",
		Platform:     PlatformIos,
		RegisteredAt: time.Date(2026, 6, 12, 9, 30, 0, 0, time.UTC),
	}

	raw, err := json.Marshal(newDeviceDocument(reg))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	want := map[string]any{
		"id":           "tok-abc",
		"userId":       "auth0|u1",
		"token":        "tok-abc",
		"platform":     "Ios",
		"registeredAt": "2026-06-12T09:30:00+00:00",
		"ttl":          float64(180 * 24 * 60 * 60),
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("doc[%q] = %v (%T), want %v (%T)", k, got[k], got[k], v, v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("doc has %d keys, want %d: %v", len(got), len(want), got)
	}
}

// TestDeviceDocument_RoundTrip rehydrates a stored document back to the domain
// value, including parsing the string platform.
func TestDeviceDocument_RoundTrip(t *testing.T) {
	t.Parallel()

	reg := DeviceRegistration{
		UserID:       "auth0|u1",
		Token:        "tok-abc",
		Platform:     PlatformAndroid,
		RegisteredAt: time.Date(2026, 6, 12, 9, 30, 0, 0, time.UTC),
	}
	raw, err := json.Marshal(newDeviceDocument(reg))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var doc deviceDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	back, err := doc.toDomain()
	if err != nil {
		t.Fatalf("toDomain: %v", err)
	}
	if back.UserID != reg.UserID || back.Token != reg.Token || back.Platform != reg.Platform {
		t.Errorf("round-trip mismatch: got %+v want %+v", back, reg)
	}
	if !back.RegisteredAt.Equal(reg.RegisteredAt) {
		t.Errorf("RegisteredAt round-trip: got %v want %v", back.RegisteredAt, reg.RegisteredAt)
	}
}

// TestDeviceDocument_RejectsUnknownPlatform mirrors .NET Enum.Parse throwing on
// an unrecognised stored platform.
func TestDeviceDocument_RejectsUnknownPlatform(t *testing.T) {
	t.Parallel()

	doc := deviceDocument{UserID: "u", Token: "t", Platform: "Windows"}
	if _, err := doc.toDomain(); err == nil {
		t.Error("toDomain with unknown platform: want error")
	}
}
