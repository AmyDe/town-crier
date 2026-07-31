// Package appstorereconcile holds the App Store Server API reconciliation
// worker mode (WORKER_MODE=appstore-reconcile): periodically polls Apple's
// Get Notification History API for a lookback window and detects any
// notificationUUID Apple sent that never reached our idempotency table —
// evidence of a missed ASSN webhook delivery (GH #1011). It mirrors
// internal/subscriptionsweep in spirit: consumer-side interfaces declared
// here, concrete collaborators injected from main(), hand-written test
// fakes, an injected now for a pinned clock, and per-item failure isolation
// so one malformed history entry never aborts the cycle.
//
// Rollout is observe-only: a detected gap is always logged and counted, but
// only actually re-applied through Applier.Process when applyEnabled is
// true. In observe mode nothing here ever marks a gap processed — the same
// gap is re-detected and re-alerted on every subsequent cycle until either
// apply mode is turned on or someone investigates.
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

// Verifier verifies and decodes an Apple StoreKit JWS without trusting that
// Apple's history API response is itself trustworthy transport — a
// malformed or unexpected entry must not crash the cycle.
// *subscriptions.JWSVerifier satisfies it.
type Verifier interface {
	VerifyAndDecode(signedPayload string) (string, error)
}

// IdempotencyChecker reports whether a notificationUUID has already been
// recorded as processed. *subscriptions.PostgresNotificationStore satisfies
// it. Deliberately narrower than the webhook path's idempotencyStore: this
// package only ever reads the table, never marks it — see the package doc.
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

// New builds a reconciliation handler. now is injected so tests pin the
// scan window; production passes time.Now.
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

// Run computes the window [now()-lookback, now()), fetches every notification
// history item Apple recorded in it, and for each: verifies+decodes the outer
// JWS (defensive re-verification, never trusting transport), extracts the
// notificationUUID, and checks whether it is already in our idempotency
// table. Any UUID not yet recorded is a gap — logged and counted regardless
// of whether applying it would actually change anything. When applyEnabled,
// a detected gap is also replayed through the Applier (which performs its own
// second, harmless idempotency check, closing any TOCTOU race against a
// concurrent live webhook delivery for the same UUID); the observe-only
// default never marks a gap processed, so it is re-detected every cycle until
// apply mode is turned on or someone investigates.
//
// A per-item verify/decode/idempotency-check failure is logged and skipped —
// it does not fail the whole cycle. A HistoryFetcher error (Apple unreachable,
// retries exhausted) is fatal to the cycle and is returned.
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

// processOne handles a single history item in isolation: a verify/decode/
// idempotency-check failure is logged and reported as no gap / not applied
// rather than propagated, so one bad entry never aborts the cycle.
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

	// A gap: Apple sent this notificationUUID and it is not yet in our
	// idempotency table, full stop — regardless of whether applying it would
	// actually change anything.
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
