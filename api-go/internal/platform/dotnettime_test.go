package platform

import (
	"encoding/json"
	"testing"
	"time"
)

// TestDotNetTime_WireFormat pins the exact System.Text.Json DateTimeOffset wire
// shape: ISO 8601 with a numeric UTC offset ("+00:00"), never Go's RFC 3339 "Z"
// suffix, with trailing fractional zeros trimmed. Every Cosmos timestamp the Go
// API writes or returns must match this so the contract-diff harness passes.
func TestDotNetTime_WireFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   time.Time
		want string
	}{
		{
			name: "whole second UTC",
			in:   time.Date(2026, 6, 12, 9, 30, 0, 0, time.UTC),
			want: `"2026-06-12T09:30:00+00:00"`,
		},
		{
			name: "far future expiry",
			in:   time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC),
			want: `"2099-12-31T00:00:00+00:00"`,
		},
		{
			name: "fractional seconds trimmed",
			in:   time.Date(2026, 6, 12, 9, 30, 0, 123400000, time.UTC),
			want: `"2026-06-12T09:30:00.1234+00:00"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := json.Marshal(DotNetTime(tc.in))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("wire format: got %s, want %s", got, tc.want)
			}
		})
	}
}

// TestDotNetTimePtr preserves nil so an absent timestamp serialises as null,
// not a zero instant.
func TestDotNetTimePtr(t *testing.T) {
	t.Parallel()

	if got := DotNetTimePtr(nil); got != nil {
		t.Errorf("DotNetTimePtr(nil): got %v, want nil", got)
	}

	ts := time.Date(2026, 6, 12, 9, 30, 0, 0, time.UTC)
	got := DotNetTimePtr(&ts)
	if got == nil {
		t.Fatal("DotNetTimePtr(&ts): got nil, want value")
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `"2026-06-12T09:30:00+00:00"`; string(raw) != want {
		t.Errorf("wire format: got %s, want %s", raw, want)
	}
}
