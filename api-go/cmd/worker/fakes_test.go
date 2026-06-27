package main

import (
	"context"
	"sync"

	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// spyZoneStore is a hand-written full watchzones.Store double used by the notify
// wiring tests. It records the FindZonesContaining coordinates so a test can
// prove the notify fan-out reaches the store through the watchzones.Store
// interface (the riskiest poll path — issue #664 Phase B). Every other method
// returns the empty path; the notify tests only drive FindZonesContaining.
type spyZoneStore struct {
	mu              sync.Mutex
	findCalls       int
	lastFindLat     float64
	lastFindLng     float64
	findReturnZones []watchzones.WatchZone
}

// Compile-time proof the spy is a genuine watchzones.Store — the exact interface
// the worker wiring threads through the notify, digest and dormant paths.
var _ watchzones.Store = (*spyZoneStore)(nil)

func newSpyZoneStore() *spyZoneStore { return &spyZoneStore{} }

func (s *spyZoneStore) FindZonesContaining(_ context.Context, latitude, longitude float64) ([]watchzones.WatchZone, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.findCalls++
	s.lastFindLat = latitude
	s.lastFindLng = longitude
	return s.findReturnZones, nil
}

func (s *spyZoneStore) GetByUserID(_ context.Context, _ string) ([]watchzones.WatchZone, error) {
	return nil, nil
}

func (s *spyZoneStore) Get(_ context.Context, _, _ string) (watchzones.WatchZone, error) {
	return watchzones.WatchZone{}, nil
}

func (s *spyZoneStore) Save(_ context.Context, _ watchzones.WatchZone) error { return nil }

func (s *spyZoneStore) Delete(_ context.Context, _, _ string) error { return nil }

func (s *spyZoneStore) DeleteAllByUserID(_ context.Context, _ string) error { return nil }

func (s *spyZoneStore) DistinctAuthorityIDs(_ context.Context) ([]int, error) { return nil, nil }
