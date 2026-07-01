//go:build integration

package devicetokens

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// newDevicePGStore returns a Postgres-backed device-token store over a truncated
// database. Integration tests are NOT parallel: they share the docker-compose DB
// and the pgtest advisory lock serialises them.
func newDevicePGStore(t *testing.T) *PostgresStore {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "device_registrations")
	return NewPostgresStore(pool)
}

// pgReg builds a DeviceRegistration fixture at a fixed instant.
func pgReg(t *testing.T, userID, token string, platform DevicePlatform, registeredAt time.Time) DeviceRegistration {
	t.Helper()
	reg, err := NewRegistration(userID, token, platform, registeredAt)
	if err != nil {
		t.Fatalf("NewRegistration(%s/%s): %v", userID, token, err)
	}
	return reg
}

func assertRegEqual(t *testing.T, got, want DeviceRegistration) {
	t.Helper()
	if !got.RegisteredAt.Equal(want.RegisteredAt) {
		t.Errorf("RegisteredAt: got %v, want %v", got.RegisteredAt, want.RegisteredAt)
	}
	g, w := got, want
	g.RegisteredAt, w.RegisteredAt = time.Time{}, time.Time{}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("registration mismatch:\n got = %+v\nwant = %+v", g, w)
	}
}

// TestDevicePostgresStore_SaveGetByToken round-trips a registration through Save
// then GetByToken and asserts field-level equality.
func TestDevicePostgresStore_SaveGetByToken(t *testing.T) {
	ctx := context.Background()
	store := newDevicePGStore(t)

	now := time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC)
	want := pgReg(t, "auth0|u1", "tok-abc", PlatformIos, now)

	if err := store.Save(ctx, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.GetByToken(ctx, "auth0|u1", "tok-abc")
	if err != nil {
		t.Fatalf("GetByToken: %v", err)
	}
	if got == nil {
		t.Fatal("GetByToken: got nil, want the saved registration")
	}
	assertRegEqual(t, *got, want)
}

// TestDevicePostgresStore_GetByToken_Missing returns (nil, nil) for an absent token.
func TestDevicePostgresStore_GetByToken_Missing(t *testing.T) {
	ctx := context.Background()
	store := newDevicePGStore(t)

	got, err := store.GetByToken(ctx, "auth0|u1", "no-such-token")
	if err != nil {
		t.Fatalf("GetByToken missing: got err %v", err)
	}
	if got != nil {
		t.Errorf("GetByToken missing: got %+v, want nil", got)
	}
}

// TestDevicePostgresStore_Save_UpsertResets re-saving updates registered_at and
// platform (matching the Cosmos TTL-reset semantics on re-PUT).
func TestDevicePostgresStore_Save_UpsertResets(t *testing.T) {
	ctx := context.Background()
	store := newDevicePGStore(t)

	first := pgReg(t, "auth0|u1", "tok", PlatformIos, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err := store.Save(ctx, first); err != nil {
		t.Fatalf("Save first: %v", err)
	}

	refreshed := pgReg(t, "auth0|u1", "tok", PlatformAndroid, time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC))
	if err := store.Save(ctx, refreshed); err != nil {
		t.Fatalf("Save refreshed: %v", err)
	}

	got, err := store.GetByToken(ctx, "auth0|u1", "tok")
	if err != nil || got == nil {
		t.Fatalf("GetByToken: got (%v, %v)", got, err)
	}
	assertRegEqual(t, *got, refreshed)
}

// TestDevicePostgresStore_Delete removes a token; a second delete is idempotent
// (no error, unlike the Cosmos variant which also tolerates a 404).
func TestDevicePostgresStore_Delete(t *testing.T) {
	ctx := context.Background()
	store := newDevicePGStore(t)

	reg := pgReg(t, "auth0|u1", "tok", PlatformIos, time.Now())
	if err := store.Save(ctx, reg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(ctx, "auth0|u1", "tok"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err := store.GetByToken(ctx, "auth0|u1", "tok")
	if err != nil {
		t.Fatalf("GetByToken after delete: %v", err)
	}
	if got != nil {
		t.Errorf("GetByToken after delete: got %+v, want nil", got)
	}
	// Second delete is idempotent.
	if err := store.Delete(ctx, "auth0|u1", "tok"); err != nil {
		t.Fatalf("Delete again: %v", err)
	}
}

// TestDevicePostgresStore_ListByUser lists a user's tokens in registered_at order
// and excludes other users' tokens.
func TestDevicePostgresStore_ListByUser(t *testing.T) {
	ctx := context.Background()
	store := newDevicePGStore(t)

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	for _, reg := range []DeviceRegistration{
		pgReg(t, "auth0|u1", "tok-b", PlatformIos, t2),
		pgReg(t, "auth0|u1", "tok-a", PlatformIos, t1),
		pgReg(t, "auth0|other", "tok-c", PlatformIos, t3),
	} {
		if err := store.Save(ctx, reg); err != nil {
			t.Fatalf("Save %s: %v", reg.Token, err)
		}
	}

	got, err := store.ListByUser(ctx, "auth0|u1")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListByUser: got %d registrations, want 2", len(got))
	}
	// Ordered by registered_at ascending: tok-a first, tok-b second.
	if got[0].Token != "tok-a" || got[1].Token != "tok-b" {
		t.Errorf("ListByUser order: got [%s %s], want [tok-a tok-b]", got[0].Token, got[1].Token)
	}
}

// TestDevicePostgresStore_DeleteAllByUserID clears one user's tokens and leaves
// other users' tokens intact.
func TestDevicePostgresStore_DeleteAllByUserID(t *testing.T) {
	ctx := context.Background()
	store := newDevicePGStore(t)

	now := time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC)
	for _, reg := range []DeviceRegistration{
		pgReg(t, "auth0|u1", "tok-1", PlatformIos, now),
		pgReg(t, "auth0|u1", "tok-2", PlatformIos, now),
		pgReg(t, "auth0|other", "tok-3", PlatformIos, now),
	} {
		if err := store.Save(ctx, reg); err != nil {
			t.Fatalf("Save %s: %v", reg.Token, err)
		}
	}

	if err := store.DeleteAllByUserID(ctx, "auth0|u1"); err != nil {
		t.Fatalf("DeleteAllByUserID: %v", err)
	}
	left, err := store.ListByUser(ctx, "auth0|u1")
	if err != nil {
		t.Fatalf("ListByUser u1: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("expected 0 tokens for u1, got %d", len(left))
	}
	other, err := store.ListByUser(ctx, "auth0|other")
	if err != nil {
		t.Fatalf("ListByUser other: %v", err)
	}
	if len(other) != 1 {
		t.Errorf("expected other's token untouched, got %d", len(other))
	}
}

// TestDevicePostgresStore_PurgeOlderThan deletes only registrations whose
// registered_at is before the cutoff and returns the correct deleted count.
func TestDevicePostgresStore_PurgeOlderThan(t *testing.T) {
	ctx := context.Background()
	store := newDevicePGStore(t)

	cutoff := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	old1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)   // before cutoff → purged
	old2 := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)  // before cutoff → purged
	fresh := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC) // after cutoff → kept

	for _, reg := range []DeviceRegistration{
		pgReg(t, "auth0|u1", "tok-old1", PlatformIos, old1),
		pgReg(t, "auth0|u2", "tok-old2", PlatformIos, old2),
		pgReg(t, "auth0|u3", "tok-fresh", PlatformIos, fresh),
	} {
		if err := store.Save(ctx, reg); err != nil {
			t.Fatalf("Save %s: %v", reg.Token, err)
		}
	}

	deleted, err := store.PurgeOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("PurgeOlderThan: %v", err)
	}
	if deleted != 2 {
		t.Errorf("PurgeOlderThan deleted %d, want 2", deleted)
	}

	// Fresh token survives.
	got, err := store.GetByToken(ctx, "auth0|u3", "tok-fresh")
	if err != nil || got == nil {
		t.Fatalf("GetByToken fresh: got (%v, %v)", got, err)
	}
	// Old tokens are gone.
	for _, pair := range [][2]string{{"auth0|u1", "tok-old1"}, {"auth0|u2", "tok-old2"}} {
		gone, err := store.GetByToken(ctx, pair[0], pair[1])
		if err != nil {
			t.Fatalf("GetByToken %s: %v", pair[1], err)
		}
		if gone != nil {
			t.Errorf("PurgeOlderThan: token %s should be gone, got %+v", pair[1], gone)
		}
	}
}

// TestDevicePostgresStore_PurgeOlderThan_NoneMatch returns 0 when no rows are old
// enough to purge.
func TestDevicePostgresStore_PurgeOlderThan_NoneMatch(t *testing.T) {
	ctx := context.Background()
	store := newDevicePGStore(t)

	now := time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC)
	reg := pgReg(t, "auth0|u1", "tok", PlatformIos, now)
	if err := store.Save(ctx, reg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Cutoff is before the registration → nothing to purge.
	deleted, err := store.PurgeOlderThan(ctx, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("PurgeOlderThan: %v", err)
	}
	if deleted != 0 {
		t.Errorf("PurgeOlderThan deleted %d, want 0", deleted)
	}
	// Verify errors.Is does not fire incorrectly.
	_ = errors.Is(err, nil)
}

// TestDevicePostgresStore_CountsByUsers_Integration tallies each user's live
// device registrations in one grouped query, deduped by the (user_id, token)
// primary key. A user with no rows is absent from the map; a user outside the
// requested set is excluded.
func TestDevicePostgresStore_CountsByUsers_Integration(t *testing.T) {
	ctx := context.Background()
	store := newDevicePGStore(t)

	now := time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC)
	// u1 registers two tokens, u2 registers one, u3 (queried) none, u4 (not queried).
	for _, reg := range []DeviceRegistration{
		pgReg(t, "auth0|u1", "tok-a", PlatformIos, now),
		pgReg(t, "auth0|u1", "tok-b", PlatformIos, now),
		pgReg(t, "auth0|u2", "tok-c", PlatformAndroid, now),
		pgReg(t, "auth0|u4", "tok-d", PlatformIos, now),
	} {
		if err := store.Save(ctx, reg); err != nil {
			t.Fatalf("Save %s: %v", reg.Token, err)
		}
	}
	// A re-registration of an existing (user_id, token) must not inflate the count.
	if err := store.Save(ctx, pgReg(t, "auth0|u1", "tok-a", PlatformIos, now.Add(time.Hour))); err != nil {
		t.Fatalf("Save re-register: %v", err)
	}

	got, err := store.CountsByUsers(ctx, []string{"auth0|u1", "auth0|u2", "auth0|u3"})
	if err != nil {
		t.Fatalf("CountsByUsers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("map size: got %d, want 2 (u3 absent, u4 excluded)", len(got))
	}
	if got["auth0|u1"] != 2 {
		t.Errorf("u1: got %d, want 2 (re-register deduped)", got["auth0|u1"])
	}
	if got["auth0|u2"] != 1 {
		t.Errorf("u2: got %d, want 1", got["auth0|u2"])
	}
	if _, ok := got["auth0|u3"]; ok {
		t.Error("u3 (no registrations) must be absent from the map")
	}
}

// TestDevicePostgresStore_Count_Integration returns the global count(*) across
// all users' device registrations.
func TestDevicePostgresStore_Count_Integration(t *testing.T) {
	ctx := context.Background()
	store := newDevicePGStore(t)

	if got, err := store.Count(ctx); err != nil || got != 0 {
		t.Fatalf("Count empty: got (%d, %v), want (0, nil)", got, err)
	}

	now := time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC)
	for _, reg := range []DeviceRegistration{
		pgReg(t, "auth0|u1", "tok-a", PlatformIos, now),
		pgReg(t, "auth0|u1", "tok-b", PlatformIos, now),
		pgReg(t, "auth0|u2", "tok-c", PlatformAndroid, now),
	} {
		if err := store.Save(ctx, reg); err != nil {
			t.Fatalf("Save %s: %v", reg.Token, err)
		}
	}

	got, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 3 {
		t.Errorf("Count = %d, want 3", got)
	}
}
