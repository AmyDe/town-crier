// Package pgpurge implements the pg-purge worker mode: a scheduled DELETE sweep
// that replaces the Cosmos time-to-live (TTL) settings for Notifications (90
// days) and DeviceRegistrations (180 days) on the Postgres backend. The Cosmos
// TTL is passive and automatic; Postgres has no native TTL so a periodic purge
// job must run in its place. The handler is wired only when STORE_BACKEND=postgres
// — on Cosmos deployments the worker.Run nil guard logs and exits 0.
package pgpurge

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// notifPurger deletes notification rows older than a cutoff. The
// *notifications.PostgresStore satisfies it via PurgeOlderThan.
type notifPurger interface {
	PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// devicePurger deletes device-registration rows older than a cutoff. The
// *devicetokens.PostgresStore satisfies it via PurgeOlderThan.
type devicePurger interface {
	PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// Handler runs one pg-purge cycle: deletes notifications older than
// notifRetention and device registrations older than deviceRetention. It
// returns the count of rows deleted from each table so the caller can record
// them as telemetry.
type Handler struct {
	notifs          notifPurger
	devices         devicePurger
	notifRetention  time.Duration
	deviceRetention time.Duration
	now             func() time.Time
	logger          *slog.Logger
}

// New wires the purge handler. notifRetention and deviceRetention are the
// look-back windows (e.g. 90*24*time.Hour and 180*24*time.Hour); now is
// injected so tests can pin the clock. Production passes time.Now and
// durations derived from platform.Config retention fields.
func New(
	notifs notifPurger,
	devices devicePurger,
	notifRetention time.Duration,
	deviceRetention time.Duration,
	now func() time.Time,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		notifs:          notifs,
		devices:         devices,
		notifRetention:  notifRetention,
		deviceRetention: deviceRetention,
		now:             now,
		logger:          logger,
	}
}

// Run deletes rows older than their respective retention windows and returns
// (notifsPurged, devicesPurged, error). Notifications are purged first; if
// that fails the device purge is skipped so the caller can surface a single
// actionable error.
func (h *Handler) Run(ctx context.Context) (int, int, error) {
	now := h.now()

	notifCutoff := now.Add(-h.notifRetention)
	n, err := h.notifs.PurgeOlderThan(ctx, notifCutoff)
	if err != nil {
		return 0, 0, fmt.Errorf("purge notifications: %w", err)
	}
	notifsPurged := int(n)

	deviceCutoff := now.Add(-h.deviceRetention)
	d, err := h.devices.PurgeOlderThan(ctx, deviceCutoff)
	if err != nil {
		return notifsPurged, 0, fmt.Errorf("purge device registrations: %w", err)
	}
	devicesPurged := int(d)

	h.logger.InfoContext(ctx, "pg-purge: rows deleted",
		"notificationsDeleted", notifsPurged,
		"deviceRegistrationsDeleted", devicesPurged,
		"notifCutoff", notifCutoff,
		"deviceCutoff", deviceCutoff)

	return notifsPurged, devicesPurged, nil
}
