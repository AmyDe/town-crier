package webhooks

import (
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestNewSecret(t *testing.T) {
	t.Parallel()
	first, second := NewSecret(), NewSecret()
	if first == second {
		t.Fatal("two secrets are equal")
	}
	body, ok := strings.CutPrefix(first, "tcwh_")
	if !ok {
		t.Fatalf("secret %q lacks the tcwh_ prefix", first)
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(raw) != 32 {
		t.Errorf("got %d random bytes, want 32", len(raw))
	}
}

func TestNewEndpoint(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 26, 14, 0, 0, 0, time.FixedZone("x", 3600))

	got, err := NewEndpoint("id-1", "user-1", "https://hooks.example.com/abc", FormatSlack, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "id-1" || got.UserID != "user-1" || got.URL != "https://hooks.example.com/abc" || got.Format != FormatSlack {
		t.Errorf("unexpected endpoint %+v", got)
	}
	if got.Status != StatusActive || got.DisabledReason != nil || got.ConsecutiveFailures != 0 {
		t.Errorf("new endpoint is not cleanly active: %+v", got)
	}
	if !strings.HasPrefix(got.Secret, "tcwh_") {
		t.Errorf("secret = %q", got.Secret)
	}
	if !got.CreatedAt.Equal(now) || got.CreatedAt.Location() != time.UTC || !got.UpdatedAt.Equal(now) {
		t.Errorf("timestamps = %v %v", got.CreatedAt, got.UpdatedAt)
	}

	if _, err := NewEndpoint("id", "u", "http://hooks.example.com", FormatJSON, now); !errors.Is(err, ErrInvalidURL) {
		t.Errorf("bad URL: err = %v", err)
	}
	if _, err := NewEndpoint("id", "u", "https://hooks.example.com", Format("teams"), now); !errors.Is(err, ErrInvalidFormat) {
		t.Errorf("bad format: err = %v", err)
	}
}

func TestParseFormat(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"json", "slack", "discord"} {
		if got, err := ParseFormat(in); err != nil || string(got) != in {
			t.Errorf("ParseFormat(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "JSON", "Discord", "teams"} {
		if _, err := ParseFormat(in); !errors.Is(err, ErrInvalidFormat) {
			t.Errorf("ParseFormat(%q) err = %v", in, err)
		}
	}
}

func TestSwitchOffReason(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		failure     FailureReason
		consecutive int
		want        DisabledReason
		wantOff     bool
	}{
		{"gone at once", FailureGone, 1, DisabledGone, true},
		{"23rd failed run", FailureTimeout, 23, "", false},
		{"24th failed run", FailureHTTPStatus, 24, DisabledFailures, true},
		{"beyond 24", FailureConnection, 25, DisabledFailures, true},
		{"gone wins over the count", FailureGone, 24, DisabledGone, true},
		{"first blocked address", FailureBlockedAddress, 1, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, off := SwitchOffReason(tc.failure, tc.consecutive)
			if got != tc.want || off != tc.wantOff {
				t.Errorf("SwitchOffReason = %q, %v, want %q, %v", got, off, tc.want, tc.wantOff)
			}
		})
	}
}

func TestEndpoint_LogsOnlyItsID(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	logger := slog.New(slog.NewJSONHandler(&out, nil))
	logger.Info("x", "endpoint", Endpoint{ID: "id-1", UserID: "user-1", URL: "https://secret.example.com/token", Secret: "tcwh_abc"})
	for _, leak := range []string{"secret.example.com", "token", "tcwh_abc", "user-1"} {
		if strings.Contains(out.String(), leak) {
			t.Errorf("log %s carries %q", out.String(), leak)
		}
	}
	if !strings.Contains(out.String(), "id-1") {
		t.Errorf("log %s lacks the endpoint id", out.String())
	}
}
