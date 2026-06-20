package polling

import (
	"context"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/authorities"
)

// CycleType selects which authority set a poll cycle walks.
type CycleType int

const (
	// CycleWatched polls only authorities a user is watching (the responsive
	// cycle).
	CycleWatched CycleType = iota
	// CycleSeed polls every pollable authority (the backfill cycle).
	CycleSeed
)

// TelemetryValue returns the span-tag string for this cycle type ("Watched" or
// "Seed").
func (c CycleType) TelemetryValue() string {
	if c == CycleSeed {
		return "Seed"
	}
	return "Watched"
}

// MinuteCycleSelector alternates between Watched and Seed cycles on a 15-minute
// boundary within each half hour: minute%30 < 15 → Watched, else Seed.
type MinuteCycleSelector struct {
	now func() time.Time
}

// NewMinuteCycleSelector wires the selector with an injected clock.
func NewMinuteCycleSelector(now func() time.Time) *MinuteCycleSelector {
	return &MinuteCycleSelector{now: now}
}

// Current returns the cycle type for the current minute.
func (s *MinuteCycleSelector) Current() CycleType {
	if s.now().Minute()%30 < 15 {
		return CycleWatched
	}
	return CycleSeed
}

// zoneAuthoritySource yields the distinct authority ids across all watch zones.
// *watchzones.CosmosStore satisfies it via DistinctAuthorityIDs.
type zoneAuthoritySource interface {
	DistinctAuthorityIDs(ctx context.Context) ([]int, error)
}

// WatchZoneAuthorityProvider returns the authorities that at least one user is
// watching.
type WatchZoneAuthorityProvider struct {
	source zoneAuthoritySource
}

// NewWatchZoneAuthorityProvider wires the provider over a zone authority source.
func NewWatchZoneAuthorityProvider(source zoneAuthoritySource) *WatchZoneAuthorityProvider {
	return &WatchZoneAuthorityProvider{source: source}
}

// ActiveAuthorityIDs returns the distinct watch-zone authority ids.
func (p *WatchZoneAuthorityProvider) ActiveAuthorityIDs(ctx context.Context) ([]int, error) {
	return p.source.DistinctAuthorityIDs(ctx)
}

// nonPollableAreaTypes are the regional aggregates and non-LPA containers PlanIt
// exposes but which never return planning applications. Polling them wastes RUs
// and skews diagnostics. Note: "Crown Dependencies" (plural, the aggregate) is
// excluded while the singular "Crown Dependency" records remain pollable.
var nonPollableAreaTypes = map[string]struct{}{
	"English Region":      {},
	"UK Nation":           {},
	"Cross Border Area":   {},
	"Metropolitan County": {},
	"Crown Dependencies":  {},
}

// allAuthorityLister yields the full static authority list. authorities.Lookup
// satisfies it via All().
type allAuthorityLister interface {
	All() []authorities.Authority
}

// AllAuthorityProvider returns every pollable authority id from the static
// authority list, filtering out the non-pollable area-type aggregates.
type AllAuthorityProvider struct {
	lister allAuthorityLister
}

// NewAllAuthorityProvider wires the provider over a static authority list.
func NewAllAuthorityProvider(lister allAuthorityLister) *AllAuthorityProvider {
	return &AllAuthorityProvider{lister: lister}
}

// ActiveAuthorityIDs returns the ids of all pollable authorities, preserving the
// lister's order.
func (p *AllAuthorityProvider) ActiveAuthorityIDs(_ context.Context) ([]int, error) {
	all := p.lister.All()
	ids := make([]int, 0, len(all))
	for _, a := range all {
		if _, nonPollable := nonPollableAreaTypes[a.AreaType]; nonPollable {
			continue
		}
		ids = append(ids, a.ID)
	}
	return ids, nil
}

// authorityProvider is the common contract the cycle-alternating provider
// dispatches to.
type authorityProvider interface {
	ActiveAuthorityIDs(ctx context.Context) ([]int, error)
}

// cycleSelector reports the current cycle type.
type cycleSelector interface {
	Current() CycleType
}

// CycleAlternatingProvider chooses the authority set per cycle: the Seed cycle
// walks all pollable authorities; every other cycle walks the watch-zone set.
type CycleAlternatingProvider struct {
	watched  authorityProvider
	all      authorityProvider
	selector cycleSelector
}

// NewCycleAlternatingProvider wires the provider with the watched/all providers
// and the cycle selector.
func NewCycleAlternatingProvider(watched, all authorityProvider, selector cycleSelector) *CycleAlternatingProvider {
	return &CycleAlternatingProvider{watched: watched, all: all, selector: selector}
}

// ActiveAuthorityIDs dispatches to the all-authority provider on a Seed cycle and
// the watch-zone provider otherwise.
func (p *CycleAlternatingProvider) ActiveAuthorityIDs(ctx context.Context) ([]int, error) {
	if p.selector.Current() == CycleSeed {
		return p.all.ActiveAuthorityIDs(ctx)
	}
	return p.watched.ActiveAuthorityIDs(ctx)
}
