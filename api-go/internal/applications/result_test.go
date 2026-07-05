package applications

import (
	"testing"
	"time"
)

// strPtr is a local helper for building *string appState keys in the breakdown
// tests.
func strPtr(s string) *string { return &s }

func TestRecentApplicationOf_IncludesLastDifferent(t *testing.T) {
	t.Parallel()
	a := testApplication(t)

	got := RecentApplicationOf(a)

	// The slim SEO projection carries lastDifferent (the DESC sort key) so the web
	// card can show a "Last updated" date that matches the list order.
	if !time.Time(got.LastDifferent).Equal(a.LastDifferent) {
		t.Errorf("LastDifferent: got %v, want %v", time.Time(got.LastDifferent), a.LastDifferent)
	}
}

// TestRecentApplicationOf_IncludesDecidedDate proves the slim SEO projection
// carries decidedDate (#819 decision 5) so a card can render "Decided 9 Jul
// 2021" alongside "Started ...", independent of the ordering-only StartDate.
func TestRecentApplicationOf_IncludesDecidedDate(t *testing.T) {
	t.Parallel()
	a := testApplication(t)

	got := RecentApplicationOf(a)

	if got.DecidedDate == nil {
		t.Fatalf("DecidedDate: got nil, want %v", *a.DecidedDate)
	}
	if !time.Time(*got.DecidedDate).Equal(*a.DecidedDate) {
		t.Errorf("DecidedDate: got %v, want %v", time.Time(*got.DecidedDate), *a.DecidedDate)
	}
}

// TestRecentApplicationOf_NilDecidedDateStaysNil proves an undecided
// application (still pending) round-trips DecidedDate as nil, not a zero date.
func TestRecentApplicationOf_NilDecidedDateStaysNil(t *testing.T) {
	t.Parallel()
	a := testApplication(t)
	a.DecidedDate = nil

	got := RecentApplicationOf(a)

	if got.DecidedDate != nil {
		t.Errorf("DecidedDate: got %v, want nil", *got.DecidedDate)
	}
}

// assertBreakdownEqual compares two breakdown slices positionally, including the
// nullable AppState pointer values.
func assertBreakdownEqual(t *testing.T, got, want []StateCount) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("breakdown length: got %d, want %d (%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Count != want[i].Count {
			t.Errorf("entry %d count: got %d, want %d", i, got[i].Count, want[i].Count)
		}
		switch {
		case want[i].AppState == nil && got[i].AppState != nil:
			t.Errorf("entry %d appState: got %q, want nil", i, *got[i].AppState)
		case want[i].AppState != nil && got[i].AppState == nil:
			t.Errorf("entry %d appState: got nil, want %q", i, *want[i].AppState)
		case want[i].AppState != nil && got[i].AppState != nil && *got[i].AppState != *want[i].AppState:
			t.Errorf("entry %d appState: got %q, want %q", i, *got[i].AppState, *want[i].AppState)
		}
	}
}
