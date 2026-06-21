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

func appWithState(t *testing.T, state *string) PlanningApplication {
	t.Helper()
	a := testApplication(t)
	a.AppState = state
	return a
}

func TestBreakdownByState_CountsRawStatesOverBoundedRead(t *testing.T) {
	t.Parallel()
	apps := []PlanningApplication{
		appWithState(t, strPtr("Permitted")),
		appWithState(t, strPtr("Rejected")),
		appWithState(t, strPtr("Permitted")),
		appWithState(t, strPtr("Permitted")),
		appWithState(t, strPtr("Rejected")),
	}

	got := breakdownByState(apps)

	// Raw appState keys are returned verbatim (the web owns label mapping); the
	// denominator is the bounded read, not the whole partition.
	want := []StateCount{
		{AppState: strPtr("Permitted"), Count: 3},
		{AppState: strPtr("Rejected"), Count: 2},
	}
	assertBreakdownEqual(t, got, want)
}

func TestBreakdownByState_OrdersByCountDescThenStateAsc(t *testing.T) {
	t.Parallel()
	apps := []PlanningApplication{
		appWithState(t, strPtr("Conditions")),
		appWithState(t, strPtr("Permitted")),
		appWithState(t, strPtr("Permitted")),
		appWithState(t, strPtr("Rejected")),
		appWithState(t, strPtr("Rejected")),
	}

	got := breakdownByState(apps)

	// Permitted and Rejected tie on 2 -> alphabetical asc; Conditions trails on 1.
	want := []StateCount{
		{AppState: strPtr("Permitted"), Count: 2},
		{AppState: strPtr("Rejected"), Count: 2},
		{AppState: strPtr("Conditions"), Count: 1},
	}
	assertBreakdownEqual(t, got, want)
}

func TestBreakdownByState_NilStateSortsLast(t *testing.T) {
	t.Parallel()
	apps := []PlanningApplication{
		appWithState(t, nil),
		appWithState(t, nil),
		appWithState(t, nil),
		appWithState(t, strPtr("Permitted")),
	}

	got := breakdownByState(apps)

	// A nil appState is a distinct bucket and, despite the higher count, sorts last
	// by the deterministic tie-break rule.
	want := []StateCount{
		{AppState: strPtr("Permitted"), Count: 1},
		{AppState: nil, Count: 3},
	}
	assertBreakdownEqual(t, got, want)
}

func TestBreakdownByState_EmptyInputIsNonNilEmptySlice(t *testing.T) {
	t.Parallel()
	got := breakdownByState(nil)
	if got == nil {
		t.Fatal("breakdownByState(nil): got nil slice, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("breakdownByState(nil): got %d entries, want 0", len(got))
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
