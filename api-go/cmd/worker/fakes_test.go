package main

import (
	"context"
	"sync"

	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// fakeCosmosItems is an in-memory store backend that satisfies both
// applications.CosmosItems and watchzones.CosmosItems, so the chooser tests can
// build a real *CosmosStore with no Cosmos dependency. The chooser tests only
// construct the stores (never call a method), so every method returns the empty
// path.
type fakeCosmosItems struct{}

func newFakeItems() *fakeCosmosItems { return &fakeCosmosItems{} }

func (f *fakeCosmosItems) ReadItem(_ context.Context, _, _ string) ([]byte, error) {
	return nil, nil
}

func (f *fakeCosmosItems) UpsertItem(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (f *fakeCosmosItems) DeleteItem(_ context.Context, _, _ string) error {
	return nil
}

func (f *fakeCosmosItems) QueryItems(_ context.Context, _, _ string, _ map[string]any) ([][]byte, error) {
	return nil, nil
}

func (f *fakeCosmosItems) QueryItemsLongRead(_ context.Context, _, _ string, _ map[string]any) ([][]byte, error) {
	return nil, nil
}

func (f *fakeCosmosItems) QueryItemsCrossPartition(_ context.Context, _ string, _ map[string]any) ([][]byte, error) {
	return nil, nil
}

func (f *fakeCosmosItems) QueryPageCrossPartition(_ context.Context, _ string, _ map[string]any, _ int, _ string) ([][]byte, string, error) {
	return nil, "", nil
}

// spyZoneStore is a hand-written full watchzones.Store double used by the notify
// wiring tests. It records the FindZonesContaining coordinates so a test can
// prove the notify fan-out reaches the flag-selected store through the
// watchzones.Store interface (the riskiest poll path — issue #664 Phase B). Every
// other method returns the empty path; the notify tests only drive
// FindZonesContaining.
type spyZoneStore struct {
	mu              sync.Mutex
	findCalls       int
	lastFindLat     float64
	lastFindLng     float64
	findReturnZones []watchzones.WatchZone
}

// Compile-time proof the spy is a genuine watchzones.Store — the exact interface
// chooseZoneStore returns and the worker wiring threads through the notify,
// digest and dormant paths.
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
