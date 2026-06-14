package polling

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/authorities"
)

func TestMinuteCycleSelector_AlternatesEvery15Minutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		minute int
		want   CycleType
	}{
		{0, CycleWatched},
		{14, CycleWatched},
		{15, CycleSeed},
		{29, CycleSeed},
		{30, CycleWatched},
		{44, CycleWatched},
		{45, CycleSeed},
		{59, CycleSeed},
	}
	for _, tc := range tests {
		now := func() time.Time {
			return time.Date(2026, 6, 14, 10, tc.minute, 0, 0, time.UTC)
		}
		sel := NewMinuteCycleSelector(now)
		if got := sel.Current(); got != tc.want {
			t.Errorf("minute %d: got %v, want %v", tc.minute, got, tc.want)
		}
	}
}

// fakeZoneAuthorities is a hand-written double for the watch-zone authority
// source the watch-zone provider depends on.
type fakeZoneAuthorities struct {
	ids []int
	err error
}

func (f fakeZoneAuthorities) DistinctAuthorityIDs(context.Context) ([]int, error) {
	return f.ids, f.err
}

func TestWatchZoneAuthorityProvider_ReturnsDistinctZoneAuthorities(t *testing.T) {
	t.Parallel()
	p := NewWatchZoneAuthorityProvider(fakeZoneAuthorities{ids: []int{5, 9, 5}})
	ids, err := p.ActiveAuthorityIDs(context.Background())
	if err != nil {
		t.Fatalf("ActiveAuthorityIDs: %v", err)
	}
	if !slices.Equal(ids, []int{5, 9, 5}) {
		t.Errorf("ids: got %v, want passthrough [5 9 5]", ids)
	}
}

func TestAllAuthorityProvider_FiltersNonPollableAreaTypes(t *testing.T) {
	t.Parallel()
	all := []authorities.Authority{
		{ID: 1, Name: "Real LPA", AreaType: "District"},
		{ID: 2, Name: "England", AreaType: "UK Nation"},
		{ID: 3, Name: "South East", AreaType: "English Region"},
		{ID: 4, Name: "Greater London", AreaType: "Metropolitan County"},
		{ID: 5, Name: "Cross Border", AreaType: "Cross Border Area"},
		{ID: 6, Name: "Channel Islands", AreaType: "Crown Dependencies"},
		{ID: 7, Name: "Jersey", AreaType: "Crown Dependency"}, // singular IS pollable
	}
	p := NewAllAuthorityProvider(staticAuthorities(all))
	ids, err := p.ActiveAuthorityIDs(context.Background())
	if err != nil {
		t.Fatalf("ActiveAuthorityIDs: %v", err)
	}
	// Only the real LPA (1) and the singular Crown Dependency (7) are pollable.
	want := []int{1, 7}
	if !slices.Equal(ids, want) {
		t.Errorf("pollable ids: got %v, want %v", ids, want)
	}
}

func TestCycleAlternatingProvider_UsesAllAuthoritiesOnSeedAndWatchedOnWatched(t *testing.T) {
	t.Parallel()
	all := NewAllAuthorityProvider(staticAuthorities([]authorities.Authority{
		{ID: 100, Name: "All A", AreaType: "District"},
		{ID: 200, Name: "All B", AreaType: "District"},
	}))
	watched := NewWatchZoneAuthorityProvider(fakeZoneAuthorities{ids: []int{200}})

	// Seed cycle (minute 15-29) -> all authorities.
	seedClock := func() time.Time { return time.Date(2026, 6, 14, 10, 20, 0, 0, time.UTC) }
	seedProvider := NewCycleAlternatingProvider(watched, all, NewMinuteCycleSelector(seedClock))
	seedIDs, err := seedProvider.ActiveAuthorityIDs(context.Background())
	if err != nil {
		t.Fatalf("seed ActiveAuthorityIDs: %v", err)
	}
	if !slices.Equal(seedIDs, []int{100, 200}) {
		t.Errorf("seed cycle: got %v, want all [100 200]", seedIDs)
	}

	// Watched cycle (minute 0-14) -> watch-zone authorities.
	watchedClock := func() time.Time { return time.Date(2026, 6, 14, 10, 5, 0, 0, time.UTC) }
	watchedProvider := NewCycleAlternatingProvider(watched, all, NewMinuteCycleSelector(watchedClock))
	watchedIDs, err := watchedProvider.ActiveAuthorityIDs(context.Background())
	if err != nil {
		t.Fatalf("watched ActiveAuthorityIDs: %v", err)
	}
	if !slices.Equal(watchedIDs, []int{200}) {
		t.Errorf("watched cycle: got %v, want watch-zone [200]", watchedIDs)
	}
}

// staticAuthorities adapts a slice into the allAuthorityLister the
// all-authority provider depends on.
type staticAuthorities []authorities.Authority

func (s staticAuthorities) All() []authorities.Authority { return s }
