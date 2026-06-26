//go:build integration

package savedapplications

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// newSavedPGStore returns a Postgres-backed saved-application store over a
// truncated database. Integration tests are NOT parallel: they share the
// docker-compose DB and the pgtest advisory lock serialises them.
func newSavedPGStore(t *testing.T) *PostgresStore {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "saved_applications")
	return NewPostgresStore(pool)
}

// pgApp builds a minimal PlanningApplication fixture.
func pgApp(name string, areaID int) applications.PlanningApplication {
	return applications.PlanningApplication{
		Name:          name,
		UID:           "uid-" + name,
		AreaName:      "Testshire",
		AreaID:        areaID,
		Address:       "1 Test Street",
		Description:   "test application",
		LastDifferent: time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC),
	}
}

// pgSavedApp is a convenience constructor for test fixtures.
func pgSavedApp(t *testing.T, userID string, app applications.PlanningApplication, savedAt time.Time) SavedApplication {
	t.Helper()
	return NewSavedApplication(userID, app, savedAt)
}

func assertSavedAppEqual(t *testing.T, got, want SavedApplication) {
	t.Helper()
	if !got.SavedAt.Equal(want.SavedAt) {
		t.Errorf("SavedAt: got %v, want %v", got.SavedAt, want.SavedAt)
	}
	g, w := got, want
	g.SavedAt, w.SavedAt = time.Time{}, time.Time{}
	// Nil-snapshot paths: compare Application pointer vs nil.
	if (g.Application == nil) != (w.Application == nil) {
		t.Errorf("Application nil mismatch: got %v, want %v", g.Application, w.Application)
		return
	}
	if g.Application != nil {
		ga, wa := *g.Application, *w.Application
		// LastDifferent round-trips through timestamptz.
		if !ga.LastDifferent.Equal(wa.LastDifferent) {
			t.Errorf("Application.LastDifferent: got %v, want %v", ga.LastDifferent, wa.LastDifferent)
		}
		ga.LastDifferent, wa.LastDifferent = time.Time{}, time.Time{}
		if !reflect.DeepEqual(ga, wa) {
			t.Errorf("Application mismatch:\n got  = %+v\nwant = %+v", ga, wa)
		}
		g.Application, w.Application = nil, nil
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("SavedApplication mismatch:\n got  = %+v\nwant = %+v", g, w)
	}
}

// TestSavedPostgresStore_SaveGetByUserID round-trips a saved application through
// Save and GetByUserID, including the embedded snapshot.
func TestSavedPostgresStore_SaveGetByUserID(t *testing.T) {
	ctx := context.Background()
	store := newSavedPGStore(t)

	app := pgApp("24/00001/FUL", 100)
	savedAt := time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC)
	want := pgSavedApp(t, "auth0|u1", app, savedAt)

	if err := store.Save(ctx, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.GetByUserID(ctx, "auth0|u1")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("GetByUserID: got %d records, want 1", len(got))
	}
	assertSavedAppEqual(t, got[0], want)
}

// TestSavedPostgresStore_Exists reports true after a save and false before.
func TestSavedPostgresStore_Exists(t *testing.T) {
	ctx := context.Background()
	store := newSavedPGStore(t)

	app := pgApp("24/00001/FUL", 100)
	sa := pgSavedApp(t, "auth0|u1", app, time.Now())

	exists, err := store.Exists(ctx, "auth0|u1", sa.ApplicationUID)
	if err != nil {
		t.Fatalf("Exists before save: %v", err)
	}
	if exists {
		t.Error("Exists before save: got true, want false")
	}

	if err := store.Save(ctx, sa); err != nil {
		t.Fatalf("Save: %v", err)
	}

	exists, err = store.Exists(ctx, "auth0|u1", sa.ApplicationUID)
	if err != nil {
		t.Fatalf("Exists after save: %v", err)
	}
	if !exists {
		t.Error("Exists after save: got false, want true")
	}
}

// TestSavedPostgresStore_Delete removes a record; a second delete is idempotent.
func TestSavedPostgresStore_Delete(t *testing.T) {
	ctx := context.Background()
	store := newSavedPGStore(t)

	app := pgApp("24/00001/FUL", 100)
	sa := pgSavedApp(t, "auth0|u1", app, time.Now())

	if err := store.Save(ctx, sa); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(ctx, "auth0|u1", sa.ApplicationUID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	exists, err := store.Exists(ctx, "auth0|u1", sa.ApplicationUID)
	if err != nil || exists {
		t.Fatalf("Exists after delete: got (%v, %v)", exists, err)
	}
	// Idempotent: second delete must not error.
	if err := store.Delete(ctx, "auth0|u1", sa.ApplicationUID); err != nil {
		t.Fatalf("Delete again: %v", err)
	}
}

// TestSavedPostgresStore_GetByUserID_OrderedAndScoped returns a user's records in
// saved_at order and excludes other users' records.
func TestSavedPostgresStore_GetByUserID_OrderedAndScoped(t *testing.T) {
	ctx := context.Background()
	store := newSavedPGStore(t)

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	// Insert out of order to exercise the ORDER BY.
	for _, sa := range []SavedApplication{
		pgSavedApp(t, "auth0|u1", pgApp("app-b", 100), t2),
		pgSavedApp(t, "auth0|u1", pgApp("app-a", 100), t1),
		pgSavedApp(t, "auth0|other", pgApp("app-c", 100), t3),
	} {
		if err := store.Save(ctx, sa); err != nil {
			t.Fatalf("Save %s: %v", sa.ApplicationUID, err)
		}
	}

	got, err := store.GetByUserID(ctx, "auth0|u1")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetByUserID: got %d records, want 2", len(got))
	}
	// Ordered by saved_at: app-a first.
	if got[0].ApplicationUID != pgSavedApp(t, "auth0|u1", pgApp("app-a", 100), t1).ApplicationUID {
		t.Errorf("first record: got %q, want app-a uid", got[0].ApplicationUID)
	}
}

// TestSavedPostgresStore_Save_Upsert verifies that re-saving the same
// (user_id, application_uid) updates the existing row (snapshot + authority_id).
func TestSavedPostgresStore_Save_Upsert(t *testing.T) {
	ctx := context.Background()
	store := newSavedPGStore(t)

	app := pgApp("24/00001/FUL", 100)
	sa := pgSavedApp(t, "auth0|u1", app, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err := store.Save(ctx, sa); err != nil {
		t.Fatalf("Save first: %v", err)
	}

	// Update the application state so the snapshot changes.
	state := "Permitted"
	app.AppState = &state
	updated := pgSavedApp(t, "auth0|u1", app, time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC))
	if err := store.Save(ctx, updated); err != nil {
		t.Fatalf("Save updated: %v", err)
	}

	got, err := store.GetByUserID(ctx, "auth0|u1")
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("GetByUserID after upsert: got %d records, want 1", len(got))
	}
	assertSavedAppEqual(t, got[0], updated)
}

// TestSavedPostgresStore_UserIDsForApplication returns the distinct user ids that
// have saved the given (applicationUID, authorityID), scoped by authority.
func TestSavedPostgresStore_UserIDsForApplication(t *testing.T) {
	ctx := context.Background()
	store := newSavedPGStore(t)

	app := pgApp("24/00001/FUL", 100)
	now := time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC)

	// Three users in authority 100 save the same application.
	for _, userID := range []string{"auth0|u1", "auth0|u2", "auth0|u3"} {
		sa := pgSavedApp(t, userID, app, now)
		if err := store.Save(ctx, sa); err != nil {
			t.Fatalf("Save %s: %v", userID, err)
		}
	}
	// One user in a different authority (200) saves an application with the same uid.
	otherApp := pgApp("24/00001/FUL", 200) // same planit_name, different authority
	sa := pgSavedApp(t, "auth0|u4", otherApp, now)
	if err := store.Save(ctx, sa); err != nil {
		t.Fatalf("Save u4 (authority 200): %v", err)
	}

	got, err := store.UserIDsForApplication(ctx, app.CanonicalUID(), 100)
	if err != nil {
		t.Fatalf("UserIDsForApplication: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("UserIDsForApplication: got %d users, want 3 (authority 100 only)", len(got))
	}
	// Authority 200 user must not appear.
	for _, uid := range got {
		if uid == "auth0|u4" {
			t.Errorf("UserIDsForApplication: auth0|u4 (authority 200) leaked into authority 100 result")
		}
	}
}

// TestSavedPostgresStore_UserIDsForApplication_EmptyForMismatch returns an empty
// result when the authority does not match any saved row.
func TestSavedPostgresStore_UserIDsForApplication_EmptyForMismatch(t *testing.T) {
	ctx := context.Background()
	store := newSavedPGStore(t)

	app := pgApp("24/00001/FUL", 100)
	sa := pgSavedApp(t, "auth0|u1", app, time.Now())
	if err := store.Save(ctx, sa); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.UserIDsForApplication(ctx, app.CanonicalUID(), 999)
	if err != nil {
		t.Fatalf("UserIDsForApplication wrong authority: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("UserIDsForApplication wrong authority: got %v, want empty", got)
	}
}

// TestSavedPostgresStore_DeleteAllByUserID clears one user's records and leaves
// other users' records intact.
func TestSavedPostgresStore_DeleteAllByUserID(t *testing.T) {
	ctx := context.Background()
	store := newSavedPGStore(t)

	now := time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC)
	for _, sa := range []SavedApplication{
		pgSavedApp(t, "auth0|u1", pgApp("app-a", 100), now),
		pgSavedApp(t, "auth0|u1", pgApp("app-b", 100), now),
		pgSavedApp(t, "auth0|other", pgApp("app-c", 100), now),
	} {
		if err := store.Save(ctx, sa); err != nil {
			t.Fatalf("Save %s: %v", sa.ApplicationUID, err)
		}
	}

	if err := store.DeleteAllByUserID(ctx, "auth0|u1"); err != nil {
		t.Fatalf("DeleteAllByUserID: %v", err)
	}
	left, err := store.GetByUserID(ctx, "auth0|u1")
	if err != nil {
		t.Fatalf("GetByUserID u1: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("expected 0 records for u1 after DeleteAll, got %d", len(left))
	}
	other, err := store.GetByUserID(ctx, "auth0|other")
	if err != nil {
		t.Fatalf("GetByUserID other: %v", err)
	}
	if len(other) != 1 {
		t.Errorf("expected other's record untouched, got %d", len(other))
	}
}

// TestSavedPostgresStore_NilSnapshot stores a record with no snapshot (nil
// Application) and confirms the round-trip returns nil Application.
func TestSavedPostgresStore_NilSnapshot(t *testing.T) {
	ctx := context.Background()
	store := newSavedPGStore(t)

	sa := SavedApplication{
		UserID:         "auth0|u1",
		ApplicationUID: "100/24/legacy",
		AuthorityID:    100,
		SavedAt:        time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC),
		Application:    nil,
	}
	if err := store.Save(ctx, sa); err != nil {
		t.Fatalf("Save nil snapshot: %v", err)
	}
	got, err := store.GetByUserID(ctx, "auth0|u1")
	if err != nil || len(got) != 1 {
		t.Fatalf("GetByUserID: got (%v, %v)", got, err)
	}
	if got[0].Application != nil {
		t.Errorf("round-trip nil snapshot: got non-nil Application %+v", got[0].Application)
	}
}
