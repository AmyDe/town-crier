package applications

import (
	"errors"
	"testing"
)

// TestPageCursor_RoundTrip proves the sort-aware keyset cursor survives an
// encode/decode round-trip for every shape: a distance keyset, a non-null
// start_date keyset, a NULL start_date keyset (the tail of a NULLS LAST scan),
// each of the four status keyset NULL-tail positions (the mixed-direction keyset
// carries an extra nullable app_state key), and the recent-activity keyset
// (a nullable activity timestamp + tiebreak, with its NULL-activity tail).
func TestPageCursor_RoundTrip(t *testing.T) {
	t.Parallel()
	sd := "2026-01-02"
	as := "Permitted"
	ts := "2026-06-15T12:30:00Z"
	tests := []struct {
		name string
		in   pageCursor
	}{
		{"distance", pageCursor{M: SortDistance, D: "123.456", N: "24/0001/FUL"}},
		{"newest non-null", pageCursor{M: SortNewest, SD: &sd, AC: "100", N: "24/0002/FUL"}},
		{"oldest non-null", pageCursor{M: SortOldest, SD: &sd, AC: "200", N: "24/0003/FUL"}},
		{"newest null tail", pageCursor{M: SortNewest, SD: nil, AC: "300", N: "24/0004/FUL"}},
		{"status state+date", pageCursor{M: SortStatus, AS: &as, SD: &sd, AC: "100", N: "24/0005/FUL"}},
		{"status date-null tail", pageCursor{M: SortStatus, AS: &as, SD: nil, AC: "100", N: "24/0006/FUL"}},
		{"status state-null tail", pageCursor{M: SortStatus, AS: nil, SD: &sd, AC: "200", N: "24/0007/FUL"}},
		{"status both-null tail", pageCursor{M: SortStatus, AS: nil, SD: nil, AC: "300", N: "24/0008/FUL"}},
		{"recent-activity non-null", pageCursor{M: SortRecentActivity, TS: &ts, AC: "100", N: "24/0009/FUL"}},
		{"recent-activity null tail", pageCursor{M: SortRecentActivity, TS: nil, AC: "400", N: "24/0010/FUL"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			token, err := encodePageCursor(tc.in)
			if err != nil {
				t.Fatalf("encodePageCursor: %v", err)
			}
			got, err := decodePageCursor(token)
			if err != nil {
				t.Fatalf("decodePageCursor: %v", err)
			}
			if got.M != tc.in.M || got.D != tc.in.D || got.AC != tc.in.AC || got.N != tc.in.N {
				t.Errorf("round-trip mismatch: got %+v, want %+v", got, tc.in)
			}
			if (got.AS == nil) != (tc.in.AS == nil) {
				t.Fatalf("AS nil mismatch: got %v, want %v", got.AS, tc.in.AS)
			}
			if got.AS != nil && *got.AS != *tc.in.AS {
				t.Errorf("AS: got %q, want %q", *got.AS, *tc.in.AS)
			}
			if (got.SD == nil) != (tc.in.SD == nil) {
				t.Fatalf("SD nil mismatch: got %v, want %v", got.SD, tc.in.SD)
			}
			if got.SD != nil && *got.SD != *tc.in.SD {
				t.Errorf("SD: got %q, want %q", *got.SD, *tc.in.SD)
			}
			if (got.TS == nil) != (tc.in.TS == nil) {
				t.Fatalf("TS nil mismatch: got %v, want %v", got.TS, tc.in.TS)
			}
			if got.TS != nil && *got.TS != *tc.in.TS {
				t.Errorf("TS: got %q, want %q", *got.TS, *tc.in.TS)
			}
		})
	}
}

// TestSort_Supported pins the server-side sort set: the five UI sorts are
// supported (distance, newest, oldest, status, recent-activity); anything else
// is rejected so the handler returns 400 rather than running an arbitrary order.
func TestSort_Supported(t *testing.T) {
	t.Parallel()
	supported := []Sort{SortDistance, SortNewest, SortOldest, SortStatus, SortRecentActivity}
	for _, s := range supported {
		if !s.Supported() {
			t.Errorf("Sort(%q).Supported() = false, want true", s)
		}
	}
	for _, s := range []Sort{"", "nonsense", "DISTANCE", "Newest", "recentactivity"} {
		if s.Supported() {
			t.Errorf("Sort(%q).Supported() = true, want false", s)
		}
	}
}

// TestDecodePageCursor_Malformed proves a non-base64 token and a base64 token
// whose payload is not the cursor JSON both surface ErrCursorInvalid, so the
// handler can map them to a clean 400.
func TestDecodePageCursor_Malformed(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"not base64url":     "!!!not-base64!!!",
		"base64 of garbage": "bm90LWpzb24", // base64url("not-json")
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := decodePageCursor(token); !errors.Is(err, ErrCursorInvalid) {
				t.Errorf("decodePageCursor(%q): got err %v, want ErrCursorInvalid", token, err)
			}
		})
	}
}
