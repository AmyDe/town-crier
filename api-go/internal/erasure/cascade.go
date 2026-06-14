// Package erasure holds the single shared GDPR Art. 17 account-erasure cascade
// run by both account-deletion entry points: the dormant-cleanup worker
// (WORKER_MODE=dormant-cleanup) and DELETE /v1/me. Both previously encoded the
// same ordered container list independently, the exact drift that caused bead
// tc-qkf2 (the HTTP handler omitted the cascade while the worker's grew ahead of
// it). Centralising the order here means a new per-user container is added once,
// not twice (bead tc-gf0g).
//
// The package is deliberately dependency-free (stdlib only) so the profiles
// package can import it without an import cycle — profiles imports erasure for
// Cascade, and erasure must never import profiles (nor dormant). The
// profile-absent tolerance that previously lived inside the cascade is injected
// as a predicate (Deleters.ProfileAbsent) so erasure needs no knowledge of any
// concrete sentinel error.
package erasure

import (
	"context"
	"fmt"
)

// ChildDeleter erases every document a single per-user container holds for the
// account being deleted. The per-container cascade stores expose this exact
// method, so the notifications / watchzones / savedapplications / devicetokens
// stores satisfy it directly; the notification-state store is bridged by
// NotificationStateChild. Each store tolerates a 404 on an individual delete
// internally, so any error returned here is real and must abort the cascade.
type ChildDeleter interface {
	DeleteAllByUserID(ctx context.Context, userID string) error
}

// ProfileDeleter erases the user profile document itself. The profile store's
// Delete satisfies it.
type ProfileDeleter interface {
	Delete(ctx context.Context, userID string) error
}

// Auth0Deleter removes the user from Auth0 via the Management (M2M) API. The real
// Auth0 client and the no-op fallback both satisfy it.
type Auth0Deleter interface {
	DeleteUser(ctx context.Context, userID string) error
}

// Deleters bundles every erasure step in the fixed order Cascade invokes them.
// ProfileAbsent reports whether a profile-delete error means "the profile was
// already gone" (a concurrent delete between scan and cascade), so the cascade
// tolerates it and still proceeds to Auth0; a nil predicate treats every
// profile-delete error as fatal.
type Deleters struct {
	Notifications       ChildDeleter
	WatchZones          ChildDeleter
	SavedApplications   ChildDeleter
	DeviceRegistrations ChildDeleter
	NotificationState   ChildDeleter
	Profile             ProfileDeleter
	Auth0               Auth0Deleter
	ProfileAbsent       func(error) bool
}

// profileAbsent reports whether err means the profile is already gone. A nil
// predicate is treated as "never absent", so every profile-delete error is
// fatal.
func (d Deleters) profileAbsent(err error) bool {
	return d.ProfileAbsent != nil && d.ProfileAbsent(err)
}

// Cascade runs the full GDPR Art. 17 erasure for one user in the fixed safety
// order: every per-user child container first (notifications, watch zones, saved
// applications, device registrations, notification-state watermark), then the
// profile document, then — last — the Auth0 user.
//
// Ordering is the safety contract: child records are removed before the profile,
// so a mid-cascade failure leaves the profile present (the account stays
// retryable rather than half-erased); Auth0 is deleted last so an Auth0
// Management-API failure can never strand un-erased Cosmos data. A profile delete
// that reports the profile is already gone (per Deleters.ProfileAbsent) is
// tolerated and the cascade proceeds to Auth0.
func Cascade(ctx context.Context, userID string, d Deleters) error {
	if err := d.Notifications.DeleteAllByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete notifications: %w", err)
	}
	if err := d.WatchZones.DeleteAllByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete watch zones: %w", err)
	}
	if err := d.SavedApplications.DeleteAllByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete saved applications: %w", err)
	}
	if err := d.DeviceRegistrations.DeleteAllByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete device registrations: %w", err)
	}
	if err := d.NotificationState.DeleteAllByUserID(ctx, userID); err != nil {
		return fmt.Errorf("delete notification state: %w", err)
	}
	if err := d.Profile.Delete(ctx, userID); err != nil && !d.profileAbsent(err) {
		return fmt.Errorf("delete profile: %w", err)
	}
	if err := d.Auth0.DeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("delete auth0 user: %w", err)
	}
	return nil
}

// byUserIDDeleter is the notification-state store's own delete contract. It holds
// one watermark document per user, so DeleteByUserID erases everything the user
// owns there.
type byUserIDDeleter interface {
	DeleteByUserID(ctx context.Context, userID string) error
}

// NotificationStateChild bridges the notification-state store to the uniform
// ChildDeleter contract so both entry points share one adapter instead of
// duplicating it. The store holds a single watermark document per user, so its
// DeleteByUserID erases everything the user owns in that container — exactly what
// DeleteAllByUserID promises.
func NotificationStateChild(s byUserIDDeleter) ChildDeleter {
	return notificationStateChild{s: s}
}

type notificationStateChild struct{ s byUserIDDeleter }

func (a notificationStateChild) DeleteAllByUserID(ctx context.Context, userID string) error {
	return a.s.DeleteByUserID(ctx, userID)
}
