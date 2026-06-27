package main

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// fakeWatchZoneStore is a hand-written watchzones.Store double. Only GetByUserID
// carries behaviour the export adapter needs; the remaining methods satisfy the
// interface so the fake can stand in for the concrete store.
type fakeWatchZoneStore struct {
	zones []watchzones.WatchZone
	err   error
}

func (f *fakeWatchZoneStore) GetByUserID(_ context.Context, _ string) ([]watchzones.WatchZone, error) {
	return f.zones, f.err
}

func (f *fakeWatchZoneStore) Get(_ context.Context, _, _ string) (watchzones.WatchZone, error) {
	return watchzones.WatchZone{}, nil
}

func (f *fakeWatchZoneStore) Save(_ context.Context, _ watchzones.WatchZone) error { return nil }

func (f *fakeWatchZoneStore) Delete(_ context.Context, _, _ string) error { return nil }

func (f *fakeWatchZoneStore) DeleteAllByUserID(_ context.Context, _ string) error { return nil }

func (f *fakeWatchZoneStore) DistinctAuthorityIDs(_ context.Context) ([]int, error) { return nil, nil }

func (f *fakeWatchZoneStore) FindZonesContaining(_ context.Context, _, _ float64) ([]watchzones.WatchZone, error) {
	return nil, nil
}

// TestWatchZoneExportReader_ReadsThroughStoreInterface proves the export adapter
// is backed by the consumer-side watchzones.Store interface, so GET /v1/me/data
// exports a user's watch zones (bead tc-s8g1). The fake is a plain
// watchzones.Store, which the adapter must accept and read through.
func TestWatchZoneExportReader_ReadsThroughStoreInterface(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeWatchZoneStore{zones: []watchzones.WatchZone{
		{ID: "zone-1", UserID: "u1", Name: "Home", Latitude: 51.5, Longitude: -0.1, RadiusMetres: 500, AuthorityID: 384, CreatedAt: created},
		{ID: "zone-2", UserID: "u1", Name: "Work", Latitude: 52.2, Longitude: -1.3, RadiusMetres: 250, AuthorityID: 471, CreatedAt: created},
	}}

	var reader profiles.WatchZoneReader = watchZoneExportReader{store: store}
	rows, err := reader.WatchZonesByUser(context.Background(), "u1")
	if err != nil {
		t.Fatalf("WatchZonesByUser: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if rows[0].ID != "zone-1" || rows[0].Name != "Home" || rows[0].RadiusMetres != 500 || rows[0].AuthorityID != 384 {
		t.Errorf("row[0] = %+v, want zone-1/Home/500/384", rows[0])
	}
	if rows[1].ID != "zone-2" || rows[1].Latitude != 52.2 || rows[1].Longitude != -1.3 {
		t.Errorf("row[1] = %+v, want zone-2 coords 52.2/-1.3", rows[1])
	}
	if time.Time(rows[0].CreatedAt) != created {
		t.Errorf("row[0].CreatedAt = %v, want %v", time.Time(rows[0].CreatedAt), created)
	}
}
