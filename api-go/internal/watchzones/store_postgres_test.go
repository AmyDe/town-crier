//go:build integration

package watchzones

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// Deterministic fixture geometry, mirroring the Phase 0 spatial proof: a fixed
// London centre with zone centres offset due north by a known number of metres.
const (
	wzCentreLon    = -0.1278
	wzCentreLat    = 51.5074
	wzMetresPerLat = 111_320.0
)

func wzLatNorth(metres float64) float64 { return wzCentreLat + metres/wzMetresPerLat }

// uuidN builds a deterministic, ordered UUID so ORDER BY id assertions are stable.
func uuidN(n int) string {
	return fmt.Sprintf("%08d-0000-0000-0000-000000000000", n)
}

// newZonePGStore returns a Postgres-backed watch-zone store over a truncated
// database. Integration tests are NOT parallel: they share the docker-compose DB.
func newZonePGStore(t *testing.T) *PostgresStore {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "applications", "watch_zones")
	return NewPostgresStore(pool)
}

// pgZone constructs a validated watch zone. authorityID must be positive so
// NewWatchZone (and therefore FindZonesContaining hydration) accepts it.
func pgZone(t *testing.T, id, userID, name string, lat, lon, radius float64, authorityID int) WatchZone {
	t.Helper()
	z, err := NewWatchZone(id, userID, name, lat, lon, radius, authorityID,
		time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC), true, false)
	if err != nil {
		t.Fatalf("NewWatchZone(%s): %v", name, err)
	}
	return z
}

func assertZoneEqual(t *testing.T, got, want WatchZone) {
	t.Helper()
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, want.CreatedAt)
	}
	g, w := got, want
	g.CreatedAt, w.CreatedAt = time.Time{}, time.Time{}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("watch zone mismatch:\n got = %+v\nwant = %+v", g, w)
	}
}

func zoneNames(zones []WatchZone) []string {
	names := make([]string, len(zones))
	for i, z := range zones {
		names[i] = z.Name
	}
	return names
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestWatchZonePostgresStore_SaveGetRoundTrip persists a fully-specified zone and
// reads it back unchanged via both Get and GetByUserID.
func TestWatchZonePostgresStore_SaveGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	want := pgZone(t, uuidN(1), "user-1", "Home", 51.5, -0.12, 500, 33)
	want.PushEnabled = true
	want.EmailInstantEnabled = true

	if err := store.Save(ctx, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(ctx, "user-1", uuidN(1))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertZoneEqual(t, got, want)

	list, err := store.GetByUserID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("GetByUserID len = %d, want 1", len(list))
	}
	assertZoneEqual(t, list[0], want)
}

// TestWatchZonePostgresStore_Get_Miss returns the ErrNotFound sentinel.
func TestWatchZonePostgresStore_Get_Miss(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	_, err := store.Get(ctx, "user-1", uuidN(404))
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get miss: got %v, want ErrNotFound", err)
	}
}

// TestWatchZonePostgresStore_Save_UpsertOnID updates the existing row when the
// same id is saved again, rather than inserting a second.
func TestWatchZonePostgresStore_Save_UpsertOnID(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	first := pgZone(t, uuidN(1), "user-1", "Old name", 51.5, -0.12, 500, 10)
	if err := store.Save(ctx, first); err != nil {
		t.Fatalf("Save first: %v", err)
	}
	second := pgZone(t, uuidN(1), "user-1", "New name", 52.0, -1.0, 750, 20)
	if err := store.Save(ctx, second); err != nil {
		t.Fatalf("Save second: %v", err)
	}

	got, err := store.Get(ctx, "user-1", uuidN(1))
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertZoneEqual(t, got, second)

	list, err := store.GetByUserID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 zone after upsert, got %d", len(list))
	}
}

// TestWatchZonePostgresStore_GetByUserID_OrderedAndScoped lists a user's zones in
// id order and excludes other users' zones.
func TestWatchZonePostgresStore_GetByUserID_OrderedAndScoped(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	for _, z := range []WatchZone{
		pgZone(t, uuidN(3), "user-1", "third", 51.5, -0.12, 500, 10),
		pgZone(t, uuidN(1), "user-1", "first", 51.5, -0.12, 500, 10),
		pgZone(t, uuidN(2), "user-1", "second", 51.5, -0.12, 500, 10),
		pgZone(t, uuidN(9), "user-2", "other", 51.5, -0.12, 500, 10),
	} {
		if err := store.Save(ctx, z); err != nil {
			t.Fatalf("Save %s: %v", z.Name, err)
		}
	}

	list, err := store.GetByUserID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	assertStrings(t, zoneNames(list), []string{"first", "second", "third"})
}

// TestWatchZonePostgresStore_Delete removes a zone; a miss returns ErrNotFound.
func TestWatchZonePostgresStore_Delete(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	z := pgZone(t, uuidN(1), "user-1", "Home", 51.5, -0.12, 500, 10)
	if err := store.Save(ctx, z); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(ctx, "user-1", uuidN(1)); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, "user-1", uuidN(1)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after delete: got %v, want ErrNotFound", err)
	}
	if err := store.Delete(ctx, "user-1", uuidN(1)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete miss: got %v, want ErrNotFound", err)
	}
}

// TestWatchZonePostgresStore_DeleteAllByUserID clears one user's zones and leaves
// other users untouched.
func TestWatchZonePostgresStore_DeleteAllByUserID(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	for _, z := range []WatchZone{
		pgZone(t, uuidN(1), "user-1", "a", 51.5, -0.12, 500, 10),
		pgZone(t, uuidN(2), "user-1", "b", 51.5, -0.12, 500, 10),
		pgZone(t, uuidN(3), "user-2", "keep", 51.5, -0.12, 500, 10),
	} {
		if err := store.Save(ctx, z); err != nil {
			t.Fatalf("Save %s: %v", z.Name, err)
		}
	}

	if err := store.DeleteAllByUserID(ctx, "user-1"); err != nil {
		t.Fatalf("DeleteAllByUserID: %v", err)
	}
	left, err := store.GetByUserID(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetByUserID user-1: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("expected 0 zones for user-1, got %d", len(left))
	}
	other, err := store.GetByUserID(ctx, "user-2")
	if err != nil {
		t.Fatalf("GetByUserID user-2: %v", err)
	}
	if len(other) != 1 {
		t.Errorf("expected user-2's zone untouched, got %d", len(other))
	}
}

// TestWatchZonePostgresStore_DistinctAuthorityIDs returns the deduplicated set of
// authority ids across every user's zones.
func TestWatchZonePostgresStore_DistinctAuthorityIDs(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	for _, z := range []WatchZone{
		pgZone(t, uuidN(1), "user-1", "a", 51.5, -0.12, 500, 10),
		pgZone(t, uuidN(2), "user-1", "b", 51.5, -0.12, 500, 10), // dup authority 10
		pgZone(t, uuidN(3), "user-2", "c", 51.5, -0.12, 500, 20),
		pgZone(t, uuidN(4), "user-3", "d", 51.5, -0.12, 500, 99),
	} {
		if err := store.Save(ctx, z); err != nil {
			t.Fatalf("Save %s: %v", z.Name, err)
		}
	}

	ids, err := store.DistinctAuthorityIDs(ctx)
	if err != nil {
		t.Fatalf("DistinctAuthorityIDs: %v", err)
	}
	if !reflect.DeepEqual(ids, []int{10, 20, 99}) {
		t.Fatalf("DistinctAuthorityIDs = %v, want [10 20 99]", ids)
	}
}

// TestWatchZonePostgresStore_FindZonesContaining proves the notify hot path:
// a zone whose circle contains the point matches; a same-centre narrower zone
// does not (radius-driven, not centre proximity); and matching crosses users and
// authorities (one GiST index, authority-agnostic).
func TestWatchZonePostgresStore_FindZonesContaining(t *testing.T) {
	ctx := context.Background()
	store := newZonePGStore(t)

	// Two zones centred 2 km north of the query point: the wide one's 3 km radius
	// reaches back to the point, the narrow one's 1 km does not.
	wide := pgZone(t, uuidN(1), "user-1", "zone-wide", wzLatNorth(2000), wzCentreLon, 3000, 10)
	narrow := pgZone(t, uuidN(2), "user-1", "zone-narrow", wzLatNorth(2000), wzCentreLon, 1000, 10)
	// A different user, different authority, centred on the point itself.
	other := pgZone(t, uuidN(3), "user-2", "zone-other", wzCentreLat, wzCentreLon, 500, 99)
	for _, z := range []WatchZone{wide, narrow, other} {
		if err := store.Save(ctx, z); err != nil {
			t.Fatalf("Save %s: %v", z.Name, err)
		}
	}

	matched, err := store.FindZonesContaining(ctx, wzCentreLat, wzCentreLon)
	if err != nil {
		t.Fatalf("FindZonesContaining: %v", err)
	}
	assertStrings(t, zoneNames(matched), []string{"zone-wide", "zone-other"})

	// The hydrated zones are valid (positive authority id) and carry their owner.
	for _, z := range matched {
		if z.AuthorityID <= 0 || z.UserID == "" {
			t.Errorf("hydrated zone invalid: %+v", z)
		}
	}
}
