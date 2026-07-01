package tc

import (
	"strings"
	"testing"
)

func intptr(n int) *int { return &n }

// TestPrintUsersTable_DynamicWidthsAndColumns verifies the list table aligns
// regardless of user-id length (Apple IDs are ~49 chars, Auth0 ~13) and renders
// the watch-zone, offer, last-active, created, notification, saved, and device
// columns. Cell values are asserted positionally via strings.Fields so the
// column ORDER is pinned, not just their presence.
func TestPrintUsersTable_DynamicWidthsAndColumns(t *testing.T) {
	t.Parallel()

	appleID := "apple|000190.0d7a75399d0649229f6de04d0f38d7aa.2023"
	authID := "auth0|u2"
	wz := 3
	page := &listUsersResponse{Items: []listUsersItem{
		{
			UserID:             appleID,
			Email:              strptr("alice@example.com"),
			Tier:               "Pro",
			WatchZoneCount:     &wz,
			CreatedAt:          strptr("2026-01-01T09:00:00Z"),
			LastActiveAt:       strptr("2026-03-04T10:00:00Z"),
			NotificationTotal:  57,
			NotificationUnread: 2,
			SavedCount:         17,
			DeviceCount:        8,
			OfferCode:          strptr("SUMMER25"),
		},
		{
			UserID:             authID,
			Email:              nil, // legacy profile, no email
			Tier:               "Free",
			WatchZoneCount:     nil, // legacy profile, no watch-zone count
			CreatedAt:          strptr("2026-02-15T00:00:00Z"),
			LastActiveAt:       strptr("2026-02-20T08:30:00Z"),
			NotificationTotal:  0,
			NotificationUnread: 0,
			SavedCount:         0,
			DeviceCount:        0,
			OfferCode:          nil, // no active offer code -> "-"
		},
	}}

	var sb strings.Builder
	printUsersTable(&sb, page)
	out := sb.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 4 {
		t.Fatalf("expected header + separator + 2 rows, got %d lines:\n%s", len(lines), out)
	}
	header := lines[0]
	row1 := lines[2]
	row2 := lines[3]

	// All column headers are present, including the three new ones.
	for _, col := range []string{"UserId", "Email", "Tier", "Offer", "WatchZones", "LastActive", "Created", "Notifs", "Saved", "Devices"} {
		if !strings.Contains(header, col) {
			t.Errorf("header missing column %q:\n%s", col, header)
		}
	}

	// Dynamic alignment: the Email column starts at the same offset on the header
	// and on every row, even though the two user ids differ in length.
	emailOffset := strings.Index(header, "Email")
	if got := strings.Index(row1, "alice@example.com"); got != emailOffset {
		t.Errorf("row1 email offset = %d, want %d (aligned with header)\n%s", got, emailOffset, out)
	}
	if got := strings.Index(row2, "(none)"); got != emailOffset {
		t.Errorf("row2 email offset = %d, want %d (aligned with header)\n%s", got, emailOffset, out)
	}

	// Alignment holds through the added columns: the Offer header and the row-1
	// offer code start at the same offset.
	if got, want := strings.Index(row1, "SUMMER25"), strings.Index(header, "Offer"); got != want {
		t.Errorf("row1 offer offset = %d, want %d (aligned with header)\n%s", got, want, out)
	}

	// Column ORDER + values, positionally. Fields splits on whitespace runs; every
	// cell is a single token, so the slice indexes map to columns:
	// 0 UserId 1 Email 2 Tier 3 Offer 4 WatchZones 5 LastActive 6 Created 7 Notifs 8 Saved 9 Devices.
	f1 := strings.Fields(row1)
	f2 := strings.Fields(row2)
	if len(f1) != 10 || len(f2) != 10 {
		t.Fatalf("expected 10 columns per row, got f1=%d f2=%d:\n%s", len(f1), len(f2), out)
	}
	if f1[3] != "SUMMER25" || f1[7] != "2/57" || f1[8] != "17" || f1[9] != "8" {
		t.Errorf("row1 columns wrong: offer=%q notifs=%q saved=%q devices=%q\n%s", f1[3], f1[7], f1[8], f1[9], out)
	}
	// Null offer code renders "-"; zero counts render "0".
	if f2[3] != "-" || f2[7] != "0/0" || f2[8] != "0" || f2[9] != "0" {
		t.Errorf("row2 columns wrong: offer=%q notifs=%q saved=%q devices=%q\n%s", f2[3], f2[7], f2[8], f2[9], out)
	}

	// Dates are truncated to the date portion (no time-of-day / "T").
	if !strings.Contains(row1, "2026-01-01") || !strings.Contains(row1, "2026-03-04") {
		t.Errorf("row1 should show created + last-active dates:\n%s", row1)
	}
	if strings.Contains(out, "T09:00:00") || strings.Contains(out, "2026-01-01T") {
		t.Errorf("dates must be truncated to the date portion, got:\n%s", out)
	}
}

// TestOfferCell covers the nil-vs-present rendering directly.
func TestOfferCell(t *testing.T) {
	t.Parallel()
	if got := offerCell(nil); got != "-" {
		t.Errorf("nil offer code: got %q, want -", got)
	}
	if got := offerCell(strptr("SPRING10")); got != "SPRING10" {
		t.Errorf("present offer code: got %q, want SPRING10", got)
	}
}

// TestWatchZonesCell covers the nil-vs-present rendering directly.
func TestWatchZonesCell(t *testing.T) {
	t.Parallel()
	if got := watchZonesCell(nil); got != "-" {
		t.Errorf("nil watch-zone: got %q, want -", got)
	}
	if got := watchZonesCell(intptr(5)); got != "5" {
		t.Errorf("present watch-zone: got %q, want 5", got)
	}
}

// TestDatePart trims an RFC3339 timestamp to its date, tolerating nil/short.
func TestDatePart(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   *string
		want string
	}{
		{"nil", nil, "-"},
		{"empty", strptr(""), "-"},
		{"rfc3339", strptr("2026-01-02T03:04:05Z"), "2026-01-02"},
		{"short", strptr("2026"), "2026"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := datePart(tc.in); got != tc.want {
				t.Errorf("datePart(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
