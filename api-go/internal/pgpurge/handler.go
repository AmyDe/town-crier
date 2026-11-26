// Package pgpurge implements the pg-purge worker mode: a scheduled DELETE sweep
// enforcing row retention for notifications, device registrations, webhook
// deliveries and the polling tables, since Postgres has no native time-to-live.
package pgpurge

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type notifPurger interface {
	PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type devicePurger interface {
	PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type deliveryPurger interface {
	PurgeDeliveriesOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// webhookDeliveryRetention is the period the privacy policy gives for webhook
// delivery records.
const webhookDeliveryRetention = 7 * 24 * time.Hour

type pollPurger interface {
	Purge(ctx context.Context, now time.Time) (map[string]int64, error)
}

// Handler runs one pg-purge cycle and returns the rows deleted per table.
type Handler struct {
	notifs          notifPurger
	devices         devicePurger
	deliveries      deliveryPurger
	notifRetention  time.Duration
	deviceRetention time.Duration
	poll            pollPurger
	now             func() time.Time
	logger          *slog.Logger
}

// New wires the purge handler. notifRetention and deviceRetention are the
// look-back windows. Webhook deliveries are always kept for seven days.
func New(
	notifs notifPurger,
	devices devicePurger,
	deliveries deliveryPurger,
	notifRetention time.Duration,
	deviceRetention time.Duration,
	now func() time.Time,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		notifs:          notifs,
		devices:         devices,
		deliveries:      deliveries,
		notifRetention:  notifRetention,
		deviceRetention: deviceRetention,
		now:             now,
		logger:          logger,
	}
}

// WithPollRetention makes Run also purge the polling tables through p.
func (h *Handler) WithPollRetention(p pollPurger) *Handler {
	h.poll = p
	return h
}

// Run purges notifications, then devices, then webhook deliveries, then poll
// data, and returns the notification and device counts purged; a failure skips
// the rest.
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

	deliveriesDeleted, err := h.deliveries.PurgeDeliveriesOlderThan(ctx, now.Add(-webhookDeliveryRetention))
	if err != nil {
		return notifsPurged, devicesPurged, fmt.Errorf("purge webhook deliveries: %w", err)
	}

	var pollDeleted map[string]int64
	if h.poll != nil {
		pollDeleted, err = h.poll.Purge(ctx, now)
		if err != nil {
			return notifsPurged, devicesPurged, fmt.Errorf("purge poll data: %w", err)
		}
	}

	h.logger.InfoContext(ctx, "pg-purge: rows deleted",
		"pollRowsDeleted", pollDeleted,
		"notificationsDeleted", notifsPurged,
		"deviceRegistrationsDeleted", devicesPurged,
		"webhookDeliveriesDeleted", deliveriesDeleted,
		"notifCutoff", notifCutoff,
		"deviceCutoff", deviceCutoff)

	return notifsPurged, devicesPurged, nil
}
