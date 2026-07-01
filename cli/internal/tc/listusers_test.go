package tc

import (
	"strings"
	"testing"
)

func intptr(n int) *int { return &n }

// TestPrintUsersTable_DynamicWidthsAndColumns verifies the list table aligns
// regardless of user-id length (Apple IDs are ~49 chars, Auth0 ~13) and renders
// the watch-zone, last-active, created, and notification columns.
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

	// All new column headers are present.
	for _, col := range []string{"UserId", "Email", "Tier", "WatchZones", "LastActive", "Created", "Notifs"} {
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

	// Watch-zone: count when present, "-" when nil.
	if !strings.Contains(row1, "3") {
		t.Errorf("row1 should show watch-zone count 3:\n%s", row1)
	}
	// Notifs: unread/total.
	if !strings.Contains(row1, "2/57") {
		t.Errorf("row1 should show notifs 2/57:\n%s", row1)
	}
	if !strings.Contains(row2, "0/0") {
		t.Errorf("row2 should show notifs 0/0:\n%s", row2)
	}

	// Dates are truncated to the date portion (no time-of-day / "T").
	if !strings.Contains(row1, "2026-01-01") || !strings.Contains(row1, "2026-03-04") {
		t.Errorf("row1 should show created + last-active dates:\n%s", row1)
	}
	if strings.Contains(out, "T09:00:00") || strings.Contains(out, "2026-01-01T") {
		t.Errorf("dates must be truncated to the date portion, got:\n%s", out)
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
