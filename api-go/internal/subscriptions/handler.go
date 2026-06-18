package subscriptions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/auth"
	"github.com/AmyDe/town-crier/api-go/internal/httputil"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

const maxBodyBytes = 1 << 20

// jwsVerifier verifies and decodes an Apple StoreKit JWS. *JWSVerifier satisfies it.
type jwsVerifier interface {
	VerifyAndDecode(signedPayload string) (string, error)
}

// profileByUser is the verify path's store: load the caller's profile by user
// id and save it back. profiles.CosmosStore satisfies it.
type profileByUser interface {
	Get(ctx context.Context, userID string) (*profiles.UserProfile, error)
	Save(ctx context.Context, p *profiles.UserProfile) error
}

// profileByTxn is the webhook path's store: App Store notifications carry no
// user id, so the subscriber is found by their Apple original transaction id
// (cross-partition). profiles.AdminStore satisfies it.
type profileByTxn interface {
	GetByOriginalTransactionID(ctx context.Context, originalTransactionID string) (*profiles.UserProfile, error)
	Save(ctx context.Context, p *profiles.UserProfile) error
}

// tierSync mirrors the .NET IAuth0ManagementClient subset used here: push the
// new tier into Auth0's app_metadata. profiles.Auth0Manager satisfies it.
type tierSync interface {
	UpdateSubscriptionTier(ctx context.Context, userID, tier string) error
}

// idempotencyStore gives the webhook at-most-once processing.
// CosmosNotificationStore satisfies it.
type idempotencyStore interface {
	IsProcessed(ctx context.Context, notificationUUID string) (bool, error)
	MarkProcessed(ctx context.Context, notificationUUID string) error
}

type handler struct {
	verifier            jwsVerifier
	profilesByUser      profileByUser
	profilesByTxn       profileByTxn
	auth0               tierSync
	idempotency         idempotencyStore
	bundleID            string
	allowedEnvironments []string
	now                 func() time.Time
	logger              *slog.Logger
}

// conflictError signals that the transaction's originalTransactionId is already
// owned by a different user's profile. The verify endpoint maps it to
// 409 transaction_already_claimed.
type conflictError struct{}

func (e *conflictError) Error() string {
	return "Transaction is already claimed by another account."
}

// envAllowed reports whether env appears in the allowlist, comparing
// case-insensitively and after trimming whitespace.
func envAllowed(env string, allowed []string) bool {
	env = strings.TrimSpace(env)
	for _, a := range allowed {
		if strings.EqualFold(env, strings.TrimSpace(a)) {
			return true
		}
	}
	return false
}

// Routes registers the authed verify endpoint and the anonymous App Store
// webhook on mux. The webhook is authenticated by the signed JWS itself
// (Apple -> API), so it is added to the wiring's anonymousPatterns.
// allowedEnvironments is the set of Apple StoreKit environments the handler
// accepts (e.g. ["Production"] for prod, ["Sandbox","Production"] for dev).
func Routes(mux *http.ServeMux, verifier jwsVerifier, profilesByUser profileByUser, profilesByTxn profileByTxn, auth0 tierSync, idempotency idempotencyStore, bundleID string, allowedEnvironments []string, now func() time.Time, logger *slog.Logger) {
	h := &handler{
		verifier:            verifier,
		profilesByUser:      profilesByUser,
		profilesByTxn:       profilesByTxn,
		auth0:               auth0,
		idempotency:         idempotency,
		bundleID:            bundleID,
		allowedEnvironments: allowedEnvironments,
		now:                 now,
		logger:              logger,
	}
	mux.HandleFunc("POST /v1/subscriptions/verify", h.verify)
	mux.HandleFunc("POST /v1/webhooks/appstore", h.webhook)
}

// verifyRequest accepts a purchase (single signedTransaction) or a restore
// (signedTransactions list); both are merged into one verification set.
type verifyRequest struct {
	SignedTransaction  string   `json:"signedTransaction"`
	SignedTransactions []string `json:"signedTransactions"`
}

func (r verifyRequest) collect() []string {
	out := make([]string, 0, 1+len(r.SignedTransactions))
	if strings.TrimSpace(r.SignedTransaction) != "" {
		out = append(out, r.SignedTransaction)
	}
	for _, jws := range r.SignedTransactions {
		if strings.TrimSpace(jws) != "" {
			out = append(out, jws)
		}
	}
	return out
}

type verifyResponse struct {
	Tier               string               `json:"tier"`
	SubscriptionExpiry *platform.DotNetTime `json:"subscriptionExpiry"`
	Entitlements       []string             `json:"entitlements"`
	WatchZoneLimit     int                  `json:"watchZoneLimit"`
}

type webhookRequest struct {
	SignedPayload string `json:"signedPayload"`
}

// userNotFoundError signals that an authenticated caller has no Cosmos profile
// — a server-side inconsistency the verify endpoint reports as 404, mirroring
// .NET's UserProfileNotFoundException message.
type userNotFoundError struct{ userID string }

func (e *userNotFoundError) Error() string {
	return fmt.Sprintf("No user profile found for user '%s'.", e.userID)
}

// verify implements POST /v1/subscriptions/verify. It mirrors the .NET endpoint
// error contract: 401 invalid_transaction (JWS failure), 400
// invalid_transaction_payload (decode / bundle mismatch / unknown product), 404
// user_not_found, 400 malformed_request (bad body).
func (h *handler) verify(w http.ResponseWriter, r *http.Request) {
	userID := auth.Subject(r.Context())

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.malformedBody(r, w)
		return
	}

	signed := req.collect()
	if len(signed) == 0 {
		h.malformedBody(r, w)
		return
	}

	result, err := h.runVerify(r.Context(), userID, signed)
	if err != nil {
		h.writeVerifyError(r, w, err)
		return
	}
	h.writeJSON(r, w, http.StatusOK, result)
}

// runVerify verifies every supplied JWS, applies the highest active entitlement
// to the caller's profile (or expires it when none is active), persists it, and
// syncs Auth0 — the VerifySubscriptionCommandHandler logic.
func (h *handler) runVerify(ctx context.Context, userID string, signedTransactions []string) (verifyResponse, error) {
	profile, err := h.profilesByUser.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, profiles.ErrNotFound) {
			return verifyResponse{}, &userNotFoundError{userID: userID}
		}
		return verifyResponse{}, fmt.Errorf("load profile %q: %w", userID, err)
	}

	now := h.now()
	highestTier := profiles.TierFree
	var highestExpiry time.Time
	var highestOriginalTxn string

	for _, signed := range signedTransactions {
		jsonStr, err := h.verifier.VerifyAndDecode(signed)
		if err != nil {
			return verifyResponse{}, err
		}
		txn, err := DecodeTransaction(jsonStr)
		if err != nil {
			return verifyResponse{}, err
		}
		if txn.BundleID != h.bundleID {
			return verifyResponse{}, &PayloadError{Message: fmt.Sprintf("Bundle ID mismatch: expected '%s', got '%s'.", h.bundleID, txn.BundleID)}
		}
		// F1: reject transactions from environments not in the allowlist.
		if !envAllowed(txn.Environment, h.allowedEnvironments) {
			return verifyResponse{}, &PayloadError{Message: fmt.Sprintf("Transaction environment '%s' is not accepted.", txn.Environment)}
		}
		// A restore may legitimately include lapsed transactions — skip them.
		if !txn.ExpiresDate.After(now) {
			continue
		}
		tier, err := TierForProduct(txn.ProductID)
		if err != nil {
			return verifyResponse{}, err
		}
		if tier > highestTier {
			highestTier = tier
			highestExpiry = txn.ExpiresDate
			highestOriginalTxn = txn.OriginalTransactionID
		}
	}

	if highestTier == profiles.TierFree {
		profile.ExpireSubscription()
	} else {
		// F2: enforce single-owner on the original transaction id. A transaction
		// signed by Apple proves nothing about which account made the purchase; we
		// reject cross-user linking to prevent one JWS from granting Pro on
		// unlimited accounts. Same user (idempotent re-verify) or ErrNotFound
		// (first-time claim) both proceed.
		existing, err := h.profilesByTxn.GetByOriginalTransactionID(ctx, highestOriginalTxn)
		switch {
		case err != nil && !errors.Is(err, profiles.ErrNotFound):
			return verifyResponse{}, fmt.Errorf("look up transaction owner %q: %w", highestOriginalTxn, err)
		case err == nil && existing.UserID != userID:
			return verifyResponse{}, &conflictError{}
		}
		profile.LinkOriginalTransactionID(highestOriginalTxn)
		profile.ActivateSubscription(highestTier, highestExpiry)
	}

	if err := h.profilesByUser.Save(ctx, profile); err != nil {
		return verifyResponse{}, fmt.Errorf("save profile %q: %w", userID, err)
	}
	if err := h.auth0.UpdateSubscriptionTier(ctx, profile.UserID, profile.Tier.String()); err != nil {
		return verifyResponse{}, fmt.Errorf("sync auth0 tier %q: %w", userID, err)
	}

	return verifyResponse{
		Tier:               profile.Tier.String(),
		SubscriptionExpiry: platform.DotNetTimePtr(profile.SubscriptionExpiry),
		Entitlements:       profile.Tier.Entitlements(),
		WatchZoneLimit:     profile.Tier.WatchZoneLimit(),
	}, nil
}

// webhook implements POST /v1/webhooks/appstore (App Store Server Notifications
// v2). Apple POSTs lifecycle events here; the call is anonymous and the signed
// JWS is the authentication. Always 200 on success (including duplicates and
// unknown subscribers), mirroring .NET.
func (h *handler) webhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	var req webhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.malformedBody(r, w)
		return
	}
	if strings.TrimSpace(req.SignedPayload) == "" {
		h.malformedBody(r, w)
		return
	}

	if err := h.runWebhook(r.Context(), req.SignedPayload); err != nil {
		h.writeWebhookError(r, w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// runWebhook verifies the outer notification JWS, dedupes it, verifies the inner
// transaction JWS, locates the subscriber cross-partition, applies the lifecycle
// event, and records the notification as processed — the
// HandleAppStoreNotificationCommandHandler logic.
func (h *handler) runWebhook(ctx context.Context, signedPayload string) error {
	outerJSON, err := h.verifier.VerifyAndDecode(signedPayload)
	if err != nil {
		return err
	}
	notification, err := DecodeNotification(outerJSON)
	if err != nil {
		return err
	}

	processed, err := h.idempotency.IsProcessed(ctx, notification.NotificationUUID)
	if err != nil {
		return fmt.Errorf("check notification idempotency: %w", err)
	}
	if processed {
		return nil
	}

	txnJSON, err := h.verifier.VerifyAndDecode(notification.SignedTransactionInfo)
	if err != nil {
		return err
	}
	txn, err := DecodeTransaction(txnJSON)
	if err != nil {
		return err
	}

	profile, err := h.profilesByTxn.GetByOriginalTransactionID(ctx, txn.OriginalTransactionID)
	switch {
	case errors.Is(err, profiles.ErrNotFound):
		profile = nil
	case err != nil:
		return fmt.Errorf("locate subscriber: %w", err)
	}

	// F1: on the webhook, an environment mismatch is swallowed — the notification
	// is still marked processed so Apple does not retry, but the profile is not
	// mutated. This mirrors the "mark processed, no state change" contract of
	// unknown-subscriber and no-change events already in this path.
	if profile != nil && envAllowed(txn.Environment, h.allowedEnvironments) {
		changed, err := applyNotification(profile, notification, txn)
		if err != nil {
			return err
		}
		if changed {
			if err := h.profilesByTxn.Save(ctx, profile); err != nil {
				return fmt.Errorf("save profile %q: %w", profile.UserID, err)
			}
			if err := h.auth0.UpdateSubscriptionTier(ctx, profile.UserID, profile.Tier.String()); err != nil {
				return fmt.Errorf("sync auth0 tier %q: %w", profile.UserID, err)
			}
		}
	}

	if err := h.idempotency.MarkProcessed(ctx, notification.NotificationUUID); err != nil {
		return fmt.Errorf("mark notification processed: %w", err)
	}
	return nil
}

// applyNotification mutates the profile per the App Store notification type and
// reports whether any state changed (a no-change event needs no save/sync).
// Mirrors the .NET ApplyNotification switch.
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
		profile.RenewSubscription(txn.ExpiresDate)
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

func (h *handler) writeVerifyError(r *http.Request, w http.ResponseWriter, err error) {
	var jwsErr *JWSVerificationError
	if errors.As(err, &jwsErr) {
		h.writeError(r, w, http.StatusUnauthorized, "invalid_transaction", jwsErr.Message)
		return
	}
	if msg, ok := clientPayloadError(err); ok {
		h.writeError(r, w, http.StatusBadRequest, "invalid_transaction_payload", msg)
		return
	}
	var notFound *userNotFoundError
	if errors.As(err, &notFound) {
		h.writeError(r, w, http.StatusNotFound, "user_not_found", notFound.Error())
		return
	}
	var conflict *conflictError
	if errors.As(err, &conflict) {
		h.writeError(r, w, http.StatusConflict, "transaction_already_claimed", conflict.Error())
		return
	}
	h.serverError(r, w, "verify subscription", err)
}

func (h *handler) writeWebhookError(r *http.Request, w http.ResponseWriter, err error) {
	var jwsErr *JWSVerificationError
	if errors.As(err, &jwsErr) {
		h.writeError(r, w, http.StatusUnauthorized, "invalid_notification", jwsErr.Message)
		return
	}
	if msg, ok := clientPayloadError(err); ok {
		h.writeError(r, w, http.StatusBadRequest, "invalid_notification_payload", msg)
		return
	}
	h.serverError(r, w, "handle appstore notification", err)
}

// clientPayloadError reports whether err is a malformed/invalid payload error
// (the Go analog of .NET catching ArgumentException -> 400), as opposed to a
// server-side failure (Cosmos, Auth0) which is a 500.
func clientPayloadError(err error) (string, bool) {
	var pe *PayloadError
	if errors.As(err, &pe) {
		return pe.Message, true
	}
	var upe *UnknownProductError
	if errors.As(err, &upe) {
		return upe.Error(), true
	}
	return "", false
}

// apiErrorResponse mirrors the .NET ApiErrorResponse { error, message }.
type apiErrorResponse struct {
	Error   string  `json:"error"`
	Message *string `json:"message"`
}

func (h *handler) writeJSON(r *http.Request, w http.ResponseWriter, status int, v any) {
	body, err := httputil.EncodeJSON(v)
	if err != nil {
		h.serverError(r, w, "encode response", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write subscriptions response", "error", err)
	}
}

func (h *handler) writeError(r *http.Request, w http.ResponseWriter, status int, code, message string) {
	msg := message
	body, err := httputil.EncodeJSON(apiErrorResponse{Error: code, Message: &msg})
	if err != nil {
		h.serverError(r, w, "encode error", err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		h.logger.ErrorContext(r.Context(), "write subscriptions error body", "error", err)
	}
}

func (h *handler) malformedBody(r *http.Request, w http.ResponseWriter) {
	h.writeError(r, w, http.StatusBadRequest, "malformed_request", "The request body is not valid JSON.")
}

func (h *handler) serverError(r *http.Request, w http.ResponseWriter, op string, err error) {
	h.logger.ErrorContext(r.Context(), "subscriptions request failed", "op", op, "error", err)
	w.WriteHeader(http.StatusInternalServerError)
}
