package profiles

import (
	"testing"
	"time"
)

// TestNewExportUserData_ZonePreferencesSortedByZoneID pins the fix for tc-zgnt:
// the GDPR export builds zonePreferences by ranging a Go map, whose iteration
// order is randomised, so without an explicit sort the array order flaked
// request-to-request. The export must emit the per-zone preferences in a stable
// order — sorted by zoneId — so two successive exports of the same profile are
// byte-identical. Building the map in reverse-sorted insertion order, and
// asserting the exported slice is forward-sorted, fails reliably until the sort
// lands (Go randomises map ranging, so an unsorted build matches the sorted
// expectation only by chance).
func TestNewExportUserData_ZonePreferencesSortedByZoneID(t *testing.T) {
	t.Parallel()

	p := &UserProfile{
		UserID:       "auth0|abc",
		Preferences:  DefaultPreferences(),
		LastActiveAt: time.Now(),
		ZonePreferences: map[string]ZonePreferences{
			"cb5224db": DefaultZonePreferences(),
			"eb39413d": DefaultZonePreferences(),
			"4abf25b2": DefaultZonePreferences(),
		},
	}

	want := []string{"4abf25b2", "cb5224db", "eb39413d"}
	export := newExportUserData(p)
	got := make([]string, 0, len(export.NotificationPreferences.ZonePreferences))
	for _, z := range export.NotificationPreferences.ZonePreferences {
		got = append(got, z.ZoneID)
	}

	if len(got) != len(want) {
		t.Fatalf("zonePreferences length: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("zonePreferences not sorted by zoneId: got %v, want %v", got, want)
		}
	}
}
