package devicetokens

import (
	"testing"
	"time"
)

func TestParsePlatform(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    DevicePlatform
		wantErr bool
	}{
		{"ios canonical", "Ios", PlatformIos, false},
		{"android canonical", "Android", PlatformAndroid, false},
		{"ios lowercase", "ios", PlatformIos, false},
		{"android mixed", "ANDROID", PlatformAndroid, false},
		{"unknown", "windows", 0, true},
		{"empty", "", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParsePlatform(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParsePlatform(%q): err=%v wantErr=%v", tc.in, err, tc.wantErr)
			}
			if err == nil && got != tc.want {
				t.Errorf("ParsePlatform(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestDevicePlatform_String pins the .NET enum names ("Ios"/"Android"), which are
// both the stored Platform value and the UseStringEnumConverter export value.
func TestDevicePlatform_String(t *testing.T) {
	t.Parallel()

	if got := PlatformIos.String(); got != "Ios" {
		t.Errorf("PlatformIos.String() = %q, want Ios", got)
	}
	if got := PlatformAndroid.String(); got != "Android" {
		t.Errorf("PlatformAndroid.String() = %q, want Android", got)
	}
}

func TestNewRegistration(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC)

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		reg, err := NewRegistration("auth0|u1", "tok-abc", PlatformIos, now)
		if err != nil {
			t.Fatalf("NewRegistration: %v", err)
		}
		if reg.UserID != "auth0|u1" || reg.Token != "tok-abc" || reg.Platform != PlatformIos {
			t.Errorf("registration fields wrong: %+v", reg)
		}
		if !reg.RegisteredAt.Equal(now) {
			t.Errorf("RegisteredAt = %v, want %v", reg.RegisteredAt, now)
		}
	})

	for _, tc := range []struct {
		name   string
		userID string
		token  string
	}{
		{"blank user", "  ", "tok"},
		{"blank token", "auth0|u1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewRegistration(tc.userID, tc.token, PlatformIos, now); err == nil {
				t.Errorf("NewRegistration(%q,%q): want error", tc.userID, tc.token)
			}
		})
	}
}

// TestRegistration_Refresh advances RegisteredAt unconditionally, mirroring
// .NET DeviceRegistration.RefreshRegistration (a re-PUT stamps the new instant
// even if it is earlier — the client's clock is authoritative).
func TestRegistration_Refresh(t *testing.T) {
	t.Parallel()

	reg, err := NewRegistration("auth0|u1", "tok", PlatformIos, time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewRegistration: %v", err)
	}
	later := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	reg.Refresh(later)
	if !reg.RegisteredAt.Equal(later) {
		t.Errorf("RegisteredAt = %v, want %v", reg.RegisteredAt, later)
	}
}
