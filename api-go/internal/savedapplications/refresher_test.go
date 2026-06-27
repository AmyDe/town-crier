package savedapplications

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeSnapshotStore struct {
	existsResult bool
	existsErr    error
	rows         []SavedApplication
	rowsErr      error
	saved        []SavedApplication
	saveErr      error
}

func (f *fakeSnapshotStore) Exists(_ context.Context, _, _ string) (bool, error) {
	return f.existsResult, f.existsErr
}

func (f *fakeSnapshotStore) GetByUserID(_ context.Context, _ string) ([]SavedApplication, error) {
	return f.rows, f.rowsErr
}

func (f *fakeSnapshotStore) Save(_ context.Context, sa SavedApplication) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, sa)
	return nil
}

func fixedNow() func() time.Time {
	return func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }
}

func TestSnapshotRefresher_NoOpWhenNotSaved(t *testing.T) {
	t.Parallel()
	store := &fakeSnapshotStore{existsResult: false}
	r := NewSnapshotRefresher(store, fixedNow())

	if err := r.RefreshSnapshot(context.Background(), user, testApp(t)); err != nil {
		t.Fatalf("RefreshSnapshot: %v", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("must not save when application is not already saved: %+v", store.saved)
	}
}

func TestSnapshotRefresher_PreservesSavedAtAndRefreshesSnapshot(t *testing.T) {
	t.Parallel()
	original := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	stale := testApp(t)
	stale.Description = "stale description"
	store := &fakeSnapshotStore{
		existsResult: true,
		rows:         []SavedApplication{NewSavedApplication(user, stale, original)},
	}
	r := NewSnapshotRefresher(store, fixedNow())

	fresh := testApp(t)
	fresh.Description = "fresh description"
	if err := r.RefreshSnapshot(context.Background(), user, fresh); err != nil {
		t.Fatalf("RefreshSnapshot: %v", err)
	}
	if len(store.saved) != 1 {
		t.Fatalf("expected one refreshed save, got %+v", store.saved)
	}
	got := store.saved[0]
	if !got.SavedAt.Equal(original) {
		t.Errorf("savedAt: got %v, want preserved %v", got.SavedAt, original)
	}
	if got.ApplicationUID != fresh.CanonicalUID() {
		t.Errorf("applicationUid: got %q, want canonical %q", got.ApplicationUID, fresh.CanonicalUID())
	}
	if got.Application == nil || got.Application.Description != "fresh description" {
		t.Errorf("snapshot not refreshed: %+v", got.Application)
	}
}

func TestSnapshotRefresher_FallsBackToNowWhenRowVanished(t *testing.T) {
	t.Parallel()
	// Row exists at the presence check but has vanished by the list read.
	store := &fakeSnapshotStore{existsResult: true, rows: nil}
	r := NewSnapshotRefresher(store, fixedNow())

	if err := r.RefreshSnapshot(context.Background(), user, testApp(t)); err != nil {
		t.Fatalf("RefreshSnapshot: %v", err)
	}
	if len(store.saved) != 1 {
		t.Fatalf("expected one save, got %+v", store.saved)
	}
	if !store.saved[0].SavedAt.Equal(fixedNow()()) {
		t.Errorf("savedAt: got %v, want now %v", store.saved[0].SavedAt, fixedNow()())
	}
}

func TestSnapshotRefresher_PropagatesStoreErrors(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("boom")
	tests := map[string]*fakeSnapshotStore{
		"exists error": {existsErr: wantErr},
		"list error":   {existsResult: true, rowsErr: wantErr},
		"save error":   {existsResult: true, saveErr: wantErr},
	}
	for name, store := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := NewSnapshotRefresher(store, fixedNow())
			err := r.RefreshSnapshot(context.Background(), user, testApp(t))
			if !errors.Is(err, wantErr) {
				t.Fatalf("got err=%v, want wrapped %v", err, wantErr)
			}
			if name != "save error" && len(store.saved) != 0 {
				t.Errorf("must not save on pre-save error: %+v", store.saved)
			}
		})
	}
}

// Compile-time assertion that *PostgresStore satisfies the refresher's consumer
// interface, so the production wiring in wiring.go stays valid.
var _ snapshotStore = (*PostgresStore)(nil)
