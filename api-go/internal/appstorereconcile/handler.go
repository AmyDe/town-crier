// Package appstorereconcile holds the App Store Server API reconciliation
// worker mode (WORKER_MODE=appstore-reconcile): it polls Apple's Get
// Notification History API for a lookback window and detects any
// notificationUUID that never reached our idempotency table, evidence of a
// missed ASSN webhook delivery.
//
// A detected gap is re-applied only when applyEnabled is true. Observe mode
// never marks a gap processed, so the same gap is re-alerted every cycle.
package appstorereconcile

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/appstoreserverapi"
	"github.com/AmyDe/town-crier/api-go/internal/subscriptions"
)

// HistoryFetcher is the consumer-side slice of the App Store Server API
// client the handler polls. *appstoreserverapi.Client satisfies it.
type HistoryFetcher interface {
	FetchNotificationHistory(ctx context.Context, start, end time.Time) ([]appstoreserverapi.NotificationHistoryItem, error)
}

// Verifier verifies and decodes an Apple StoreKit JWS.
// *subscriptions.JWSVerifier satisfies it.
type Verifier interface {
	VerifyAndDecode(signedPayload string) (string, error)
}

// IdempotencyChecker reports whether a notificationUUID has already been
// recorded as processed. *subscriptions.PostgresNotificationStore satisfies
// it. It is read-only by design: this package never marks the table.
type IdempotencyChecker interface {
	IsProcessed(ctx context.Context, notificationUUID string) (bool, error)
}

// Applier applies one already-fetched, not-yet-verified notification JWS —
// the same choke point the live webhook uses. *subscriptions.NotificationProcessor
// satisfies it. Process re-verifies and re-checks idempotency internally, so
// calling it here can never double-apply a notification a concurrent live
// webhook delivery is also handling.
type Applier interface {
	Process(ctx context.Context, signedPayload string) (wasNew bool, err error)
}

// Handler runs one reconciliation cycle.
type Handler struct {
	fetcher      HistoryFetcher
	verifier     Verifier
	idempotency  IdempotencyChecker
	applier      Applier
	lookback     time.Duration
	applyEnabled bool
	logger       *slog.Logger
	now          func() time.Time
}

// New builds a reconciliation handler. now anchors the scan window.
func New(fetcher HistoryFetcher, verifier Verifier, idempotency IdempotencyChecker, applier Applier, lookback time.Duration, applyEnabled bool, logger *slog.Logger, now func() time.Time) *Handler {
	return &Handler{
		fetcher:      fetcher,
		verifier:     verifier,
		idempotency:  idempotency,
		applier:      applier,
		lookback:     lookback,
		applyEnabled: applyEnabled,
		logger:       logger,
		now:          now,
	}
}

// Run scans [now()-lookback, now()). A verified notificationUUID missing from
// the idempotency table is a gap, logged and counted whether or not applying
// it would change anything, and replayed through the Applier only when
// applyEnabled. A per-item failure is logged and skipped; a HistoryFetcher
// error is returned.
func (h *Handler) Run(ctx context.Context) (scanned, gaps, applied int, err error) {
	end := h.now()
	start := end.Add(-h.lookback)

	items, err := h.fetcher.FetchNotificationHistory(ctx, start, end)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("fetch notification history: %w", err)
	}
	scanned = len(items)

	for _, item := range items {
		gap, wasApplied := h.processOne(ctx, item)
		if gap {
			gaps++
		}
		if wasApplied {
			applied++
		}
	}

	h.logger.InfoContext(ctx, "app store reconcile cycle complete",
		"scanned", scanned, "gaps", gaps, "applied", applied, "applyEnabled", h.applyEnabled)
	return scanned, gaps, applied, nil
}

func (h *Handler) processOne(ctx context.Context, item appstoreserverapi.NotificationHistoryItem) (gap, applied bool) {
	outerJSON, err := h.verifier.VerifyAndDecode(item.SignedPayload)
	if err != nil {
		h.logger.WarnContext(ctx, "app store reconcile: skipping unverifiable history item", "error", err)
		return false, false
	}
	notification, err := subscriptions.DecodeNotification(outerJSON)
	if err != nil {
		h.logger.WarnContext(ctx, "app store reconcile: skipping malformed history item", "error", err)
		return false, false
	}

	processed, err := h.idempotency.IsProcessed(ctx, notification.NotificationUUID)
	if err != nil {
		h.logger.WarnContext(ctx, "app store reconcile: skipping item; idempotency check failed",
			"notificationUuid", notification.NotificationUUID, "error", err)
		return false, false
	}
	if processed {
		return false, false
	}

	h.logger.WarnContext(ctx, "app store reconcile: gap detected",
		"notificationUuid", notification.NotificationUUID,
		"notificationType", notification.NotificationType,
		"subtype", notification.Subtype,
	)

	if !h.applyEnabled {
		return true, false
	}

	wasNew, err := h.applier.Process(ctx, item.SignedPayload)
	if err != nil {
		h.logger.WarnContext(ctx, "app store reconcile: apply failed for detected gap",
			"notificationUuid", notification.NotificationUUID, "error", err)
		return true, false
	}
	return true, wasNew
}
