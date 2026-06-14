package planit

import (
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		header string
		want   time.Duration
		wantOK bool
	}{
		{"empty", "", 0, false},
		{"whitespace", "   ", 0, false},
		{"delta seconds", "120", 120 * time.Second, true},
		{"zero seconds", "0", 0, true},
		{"negative seconds", "-5", 0, false},
		{"non-numeric garbage", "soon", 0, false},
		{"http-date future", "Wed, 14 Jun 2026 12:02:00 GMT", 2 * time.Minute, true},
		{"http-date past clamps to zero", "Wed, 14 Jun 2026 11:58:00 GMT", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ParseRetryAfter(tc.header, now)
			if ok != tc.wantOK {
				t.Fatalf("ParseRetryAfter(%q): ok=%v, want %v", tc.header, ok, tc.wantOK)
			}
			if ok && got != tc.want {
				t.Errorf("ParseRetryAfter(%q): got %v, want %v", tc.header, got, tc.want)
			}
		})
	}
}
