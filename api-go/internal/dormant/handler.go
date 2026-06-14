// Package dormant holds the dormant-account cleanup worker mode
// (WORKER_MODE=dormant-cleanup): once a day it finds user accounts inactive for
// the retention window and runs the full GDPR erasure cascade for each — deleting
// the user's notifications, watch zones, saved applications, device
// registrations, and notification-state watermark from Cosmos, then the profile,
// then the Auth0 user via the Management (M2M) API.
//
// It ports the .NET DormantAccountCleanupCommandHandler + DeleteUserProfile
// CommandHandler (epic tc-wad3, bead tc-dwcq) following idiomatic Go:
// consumer-side interfaces declared here, concrete stores injected from main(),
// and hand-written test fakes. The 12-month retention window is a code constant
// (not config) so the privacy policy's "12 months of inactivity" promise is
// enforced uniformly (UK GDPR Art. 5(1)(e), ADR 0023).
package dormant

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// retentionMonths is the inactivity window after which an account is erased. It
// is a constant, not configuration, so the retention promise is enforced
// uniformly in code, mirroring .NET's DormantAccountCleanupCommandHandler.
const retentionMonths = 12

// Finder returns the dormant-account set: every profile last active strictly
// before the cutoff. profiles.AdminStore satisfies it via Dormant.
type Finder interface {
	Dormant(ctx context.Context, cutoff time.Time) ([]*profiles.UserProfile, error)
}

// The cascade interfaces below are the consumer-side slices of each per-container
// store the erasure needs. Each is a single method so a store satisfies only the
// contract the cascade actually uses; the concrete stores
// (notifications.DeleteStore, watchzones.CosmosStore, etc.) satisfy them
// structurally.

// NotificationDeleter erases a user's notifications. notifications.DeleteStore
// satisfies it.
type NotificationDeleter interface {
	DeleteAllNotifications(ctx context.Context, userID string) error
}

// WatchZoneDeleter erases a user's watch zones. watchzones.CosmosStore satisfies
// it (via an adapter in main, since its method is DeleteAllByUserID).
type WatchZoneDeleter interface {
	DeleteAllWatchZones(ctx context.Context, userID string) error
}

// SavedApplicationDeleter erases a user's saved applications.
type SavedApplicationDeleter interface {
	DeleteAllSavedApplications(ctx context.Context, userID string) error
}

// DeviceRegistrationDeleter erases a user's device registrations.
type DeviceRegistrationDeleter interface {
	DeleteAllDeviceRegistrations(ctx context.Context, userID string) error
}

// NotificationStateDeleter erases a user's notification-state watermark.
type NotificationStateDeleter interface {
	DeleteNotificationState(ctx context.Context, userID string) error
}

// ProfileDeleter erases the user profile document itself.
type ProfileDeleter interface {
	DeleteProfile(ctx context.Context, userID string) error
}

// Auth0Deleter removes the user from Auth0 via the Management (M2M) API. Both the
// real profiles.Auth0Client and profiles.NoOpAuth0Client satisfy it; the no-op is
// wired when the Auth0 M2M credentials are absent (local/dev), mirroring .NET's
// NoOpAuth0ManagementClient.
type Auth0Deleter interface {
	DeleteUser(ctx context.Context, userID string) error
}

// Stores bundles the per-step deleters so the constructor signature stays
// readable. main() builds it from the concrete stores; the test substitutes a
// single recorder that satisfies every interface.
type Stores struct {
	Notifications       NotificationDeleter
	WatchZones          WatchZoneDeleter
	SavedApplications   SavedApplicationDeleter
	DeviceRegistrations DeviceRegistrationDeleter
	NotificationState   NotificationStateDeleter
	Profiles            ProfileDeleter
	Auth0               Auth0Deleter
}

// Handler runs one dormant-cleanup cycle.
type Handler struct {
	finder Finder
	stores Stores
	logger *slog.Logger
	now    func() time.Time
}

// New builds a dormant-cleanup handler. now is injected so tests pin the cutoff;
// production passes time.Now.
func New(finder Finder, stores Stores, logger *slog.Logger, now func() time.Time) *Handler {
	return &Handler{finder: finder, stores: stores, logger: logger, now: now}
}

// Run executes one cleanup cycle and returns the number of accounts fully erased.
// It scans for dormant accounts, then runs the erasure cascade for each. A child
// step that fails leaves the profile intact (so the next daily run retries) and
// the account is not counted; a profile that has already been deleted by a
// concurrent caller is tolerated and still counted (its end state is achieved).
// A scan failure is fatal to the cycle; per-account failures are logged and the
// run continues, mirroring .NET's per-account try/catch.
func (h *Handler) Run(ctx context.Context) (int, error) {
	cutoff := h.now().AddDate(0, -retentionMonths, 0)
	dormant, err := h.finder.Dormant(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("find dormant accounts: %w", err)
	}

	deleted := 0
	for _, p := range dormant {
		if err := h.erase(ctx, p.UserID); err != nil {
			h.logger.ErrorContext(ctx, "dormant account erasure failed; will retry next cycle", "user", p.UserID, "error", err)
			continue
		}
		deleted++
	}
	h.logger.InfoContext(ctx, "dormant cleanup cycle complete", "scanned", len(dormant), "deleted", deleted)
	return deleted, nil
}

// erase runs the full erasure cascade for one user. Child records are removed
// before the profile so a mid-cascade failure leaves the profile present and the
// account retryable, mirroring the .NET ordering. A 404 on the profile delete is
// tolerated (the profile was removed concurrently), but a 404 on a child step is
// already absorbed inside each store, so any error from a child step is real and
// aborts the cascade for this account.
func (h *Handler) erase(ctx context.Context, userID string) error {
	if err := h.stores.Notifications.DeleteAllNotifications(ctx, userID); err != nil {
		return fmt.Errorf("delete notifications: %w", err)
	}
	if err := h.stores.WatchZones.DeleteAllWatchZones(ctx, userID); err != nil {
		return fmt.Errorf("delete watch zones: %w", err)
	}
	if err := h.stores.SavedApplications.DeleteAllSavedApplications(ctx, userID); err != nil {
		return fmt.Errorf("delete saved applications: %w", err)
	}
	if err := h.stores.DeviceRegistrations.DeleteAllDeviceRegistrations(ctx, userID); err != nil {
		return fmt.Errorf("delete device registrations: %w", err)
	}
	if err := h.stores.NotificationState.DeleteNotificationState(ctx, userID); err != nil {
		return fmt.Errorf("delete notification state: %w", err)
	}
	if err := h.stores.Profiles.DeleteProfile(ctx, userID); err != nil && !errors.Is(err, profiles.ErrNotFound) {
		return fmt.Errorf("delete profile: %w", err)
	}
	if err := h.stores.Auth0.DeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("delete auth0 user: %w", err)
	}
	return nil
}
