package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// NotificationProcessor applies one already-fetched, not-yet-verified App
// Store Server Notification JWS: verify (outer + inner), dedupe, apply the
// lifecycle event, save, sync Auth0, and mark processed. Both the live
// POST /v1/webhooks/appstore handler and the appstorereconcile worker call
// this — the single choke point where idempotency is enforced, so recovering
// a missed notification through reconciliation can never double-apply it
// against a concurrent live delivery.
type NotificationProcessor struct {
	verifier            jwsVerifier
	profilesByTxn       profileByTxn
	auth0               tierSync
	idempotency         idempotencyStore
	allowedEnvironments []string
	logger              *slog.Logger
}

// NewNotificationProcessor builds a processor over the given collaborators.
func NewNotificationProcessor(verifier jwsVerifier, profilesByTxn profileByTxn, auth0 tierSync, idempotency idempotencyStore, allowedEnvironments []string, logger *slog.Logger) *NotificationProcessor {
	return &NotificationProcessor{
		verifier:            verifier,
		profilesByTxn:       profilesByTxn,
		auth0:               auth0,
		idempotency:         idempotency,
		allowedEnvironments: allowedEnvironments,
		logger:              logger,
	}
}

// Process verifies the outer notification JWS, dedupes it, verifies the inner
// transaction JWS, locates the subscriber cross-partition, applies the
// lifecycle event, and records the notification as processed — the
// HandleAppStoreNotificationCommandHandler logic (formerly handler.runWebhook).
// wasNew reports whether this call actually mutated the profile — false for an
// already-processed duplicate, an unknown subscriber, an environment mismatch,
// or a no-change event (e.g. a downgrade DID_CHANGE_RENEWAL_PREF). It is
// informational only; callers must not use it to decide whether to retry.
func (p *NotificationProcessor) Process(ctx context.Context, signedPayload string) (bool, error) {
	outerJSON, err := p.verifier.VerifyAndDecode(signedPayload)
	if err != nil {
		return false, err
	}
	notification, err := DecodeNotification(outerJSON)
	if err != nil {
		return false, err
	}

	// Stamp the notification type on the live request span as soon as it's
	// decoded, before any idempotency/ignore logic runs — visibility into what
	// Apple actually sent is the point, including for notifications this
	// processor goes on to ignore (unknown subscriber, environment mismatch).
	// These are enum strings, no PII.
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("assn.notification_type", notification.NotificationType))
	if notification.Subtype != "" {
		span.SetAttributes(attribute.String("assn.subtype", notification.Subtype))
	}

	processed, err := p.idempotency.IsProcessed(ctx, notification.NotificationUUID)
	if err != nil {
		return false, fmt.Errorf("check notification idempotency: %w", err)
	}
	if processed {
		return false, nil
	}

	txnJSON, err := p.verifier.VerifyAndDecode(notification.SignedTransactionInfo)
	if err != nil {
		return false, err
	}
	txn, err := DecodeTransaction(txnJSON)
	if err != nil {
		return false, err
	}

	profile, err := p.profilesByTxn.GetByOriginalTransactionID(ctx, txn.OriginalTransactionID)
	switch {
	case errors.Is(err, profiles.ErrNotFound):
		profile = nil
	case err != nil:
		return false, fmt.Errorf("locate subscriber: %w", err)
	}

	// F1: on the webhook, an environment mismatch is swallowed — the notification
	// is still marked processed so Apple does not retry, but the profile is not
	// mutated. This mirrors the "mark processed, no state change" contract of
	// unknown-subscriber and no-change events already in this path.
	wasNew := false
	if profile != nil && envAllowed(txn.Environment, p.allowedEnvironments) {
		changed, err := applyNotification(profile, notification, txn)
		if err != nil {
			return false, err
		}
		if changed {
			if err := p.profilesByTxn.Save(ctx, profile); err != nil {
				return false, fmt.Errorf("save profile %q: %w", profile.UserID, err)
			}
			if err := p.auth0.UpdateSubscriptionTier(ctx, profile.UserID, profile.Tier.String()); err != nil {
				return false, fmt.Errorf("sync auth0 tier %q: %w", profile.UserID, err)
			}
			wasNew = true
		}
	}

	if err := p.idempotency.MarkProcessed(ctx, notification.NotificationUUID); err != nil {
		return false, fmt.Errorf("mark notification processed: %w", err)
	}
	return wasNew, nil
}

// applyNotification mutates the profile per the App Store notification type and
// reports whether any state changed (a no-change event needs no save/sync).
func applyNotification(profile *profiles.UserProfile, notification DecodedNotification, txn DecodedTransaction) (bool, error) {
	switch notification.NotificationType {
	case "SUBSCRIBED", "OFFER_REDEEMED":
		tier, err := TierForProduct(txn.ProductID)
		if err != nil {
			return false, err
		}
		profile.ActivateSubscription(tier, txn.ExpiresDate)
		return true, nil

	case "DID_RENEW":
		tier, err := TierForProduct(txn.ProductID)
		if err != nil {
			return false, err
		}
		profile.ActivateSubscription(tier, txn.ExpiresDate)
		return true, nil

	case "DID_CHANGE_RENEWAL_PREF":
		if notification.Subtype == "UPGRADE" {
			tier, err := TierForProduct(txn.ProductID)
			if err != nil {
				return false, err
			}
			profile.ActivateSubscription(tier, txn.ExpiresDate)
			return true, nil
		}
		// DOWNGRADE: no state change — it takes effect at the next renewal.
		return false, nil

	case "DID_FAIL_TO_RENEW":
		if notification.Subtype == "GRACE_PERIOD" {
			profile.EnterGracePeriod(txn.ExpiresDate)
			return true, nil
		}
		profile.ExpireSubscription()
		return true, nil

	case "EXPIRED", "GRACE_PERIOD_EXPIRED", "REFUND", "REVOKE":
		profile.ExpireSubscription()
		return true, nil

	default:
		// TEST, PRICE_INCREASE, REFUND_DECLINED, etc. — ignore.
		return false, nil
	}
}
