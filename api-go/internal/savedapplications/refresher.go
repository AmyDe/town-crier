package savedapplications

import (
	"context"
	"fmt"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// snapshotStore is the slice of the saved-application store the refresher needs.
type snapshotStore interface {
	Exists(ctx context.Context, userID, applicationUID string) (bool, error)
	GetByUserID(ctx context.Context, userID string) ([]SavedApplication, error)
	Save(ctx context.Context, sa SavedApplication) error
}

// SnapshotRefresher implements refresh-on-tap: when a user views an application
// they have already saved, re-embed the fresh snapshot into their saved row so
// the saved list self-heals on the items they actually engage with.
type SnapshotRefresher struct {
	store snapshotStore
	now   func() time.Time
}

// NewSnapshotRefresher returns a refresher over the given saved-application store.
func NewSnapshotRefresher(store snapshotStore, now func() time.Time) *SnapshotRefresher {
	return &SnapshotRefresher{store: store, now: now}
}

// RefreshSnapshot re-saves the user's saved row for app with a fresh embedded
// snapshot, keyed on the canonical {areaId}/{name} uid. It is a no-op when the
// user has not saved the application. The original SavedAt is preserved by
// reading the existing row; if that row has vanished mid-flight the current time
// is used. Callers treat this as a best-effort side effect — an error here must
// never fail the user's read.
func (r *SnapshotRefresher) RefreshSnapshot(ctx context.Context, userID string, app applications.PlanningApplication) error {
	// Saved rows are keyed on the canonical uid, not the master record's raw uid.
	canonicalUID := app.CanonicalUID()
	exists, err := r.store.Exists(ctx, userID, canonicalUID)
	if err != nil {
		return fmt.Errorf("check saved snapshot %q: %w", canonicalUID, err)
	}
	if !exists {
		return nil
	}

	savedAt := r.now()
	rows, err := r.store.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("load saved rows for refresh: %w", err)
	}
	for _, row := range rows {
		if row.ApplicationUID == canonicalUID {
			savedAt = row.SavedAt
			break
		}
	}

	if err := r.store.Save(ctx, NewSavedApplication(userID, app, savedAt)); err != nil {
		return fmt.Errorf("save refreshed snapshot %q: %w", canonicalUID, err)
	}
	return nil
}
