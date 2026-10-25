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
	"github.com/AmyDe/town-crier/api-go/internal/subscriptionevents"
)

const maxBodyBytes = 1 << 20

// maxWebhookPayloadBytes caps SignedPayload before JWS verification: a real
// Apple notification is roughly 2–4 KB, so anything over 32 KB is rejected
// before the expensive X.509 chain check.
const maxWebhookPayloadBytes = 32 * 1024

// jwsVerifier verifies and decodes an Apple StoreKit JWS. *JWSVerifier satisfies it.
type jwsVerifier interface {
	VerifyAndDecode(signedPayload string) (string, error)
}

// profileByUser is the verify path's store: load the caller's profile by user
// id and save it back.
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

// tierSync is the consumer-side interface for Auth0 tier sync: push the new tier
// into Auth0's app_metadata. profiles.Auth0Manager satisfies it.
type tierSync interface {
	UpdateSubscriptionTier(ctx context.Context, userID, tier string) error
}

// idempotencyStore gives the webhook at-most-once processing.
type idempotencyStore interface {
	IsProcessed(ctx context.Context, notificationUUID string) (bool, error)
	MarkProcessed(ctx context.Context, notificationUUID string) error
}

// subscriptionNotifier is told about each successful tier-changing save.
type subscriptionNotifier interface {
	Changed(ctx context.Context, before, after subscriptionevents.Snapshot, source subscriptionevents.Source)
}

type handler struct {
	verifier       jwsVerifier
	profilesByUser profileByUser
	// profilesByTxn is used directly by the verify path's ownership check
	// (runVerify); the webhook path reaches the same store through processor.
	profilesByTxn       profileByTxn
	auth0               tierSync
	bundleID            string
	allowedEnvironments []string
	now                 func() time.Time
	logger              *slog.Logger
	processor           *NotificationProcessor
	notifier            subscriptionNotifier
}

// conflictError signals that the transaction's originalTransactionId is already
// owned by a different user's profile. The verify endpoint maps it to
// 409 transaction_already_claimed.
type conflictError struct{}

func (e *conflictError) Error() string {
	return "Transaction is already claimed by another account."
}

// isCompactJWS is a cheap shape check (three non-empty dot-separated segments,
// no decoding) run before expensive crypto.
func isCompactJWS(s string) bool {
	first := strings.Index(s, ".")
	if first <= 0 {
		return false
	}
	rest := s[first+1:]
	second := strings.Index(rest, ".")
	if second <= 0 {
		return false
	}
	third := rest[second+1:]
	return len(third) > 0 && !strings.Contains(third, ".")
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
func Routes(mux *http.ServeMux, verifier jwsVerifier, profilesByUser profileByUser, profilesByTxn profileByTxn, auth0 tierSync, idempotency idempotencyStore, bundleID string, allowedEnvironments []string, notifier subscriptionNotifier, now func() time.Time, logger *slog.Logger) {
	h := &handler{
		verifier:            verifier,
		profilesByUser:      profilesByUser,
		profilesByTxn:       profilesByTxn,
		auth0:               auth0,
		bundleID:            bundleID,
		allowedEnvironments: allowedEnvironments,
		now:                 now,
		logger:              logger,
		notifier:            notifier,
		processor:           NewNotificationProcessor(verifier, profilesByTxn, auth0, idempotency, bundleID, allowedEnvironments, notifier, logger),
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

// userNotFoundError signals that an authenticated caller has no profile
// — a server-side inconsistency the verify endpoint reports as 404.
type userNotFoundError struct{ userID string }

func (e *userNotFoundError) Error() string {
	return fmt.Sprintf("No user profile found for user '%s'.", e.userID)
}

// verify implements POST /v1/subscriptions/verify. Error contract: 401
// invalid_transaction (JWS failure), 400 invalid_transaction_payload (decode /
// bundle mismatch / unknown product), 404 user_not_found, 400 malformed_request
// (bad body).
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

// lifetimeCandidate is the highest lifetime purchase seen while verifying a
// set of transactions. The zero value means none was seen.
type lifetimeCandidate struct {
	tier                  profiles.SubscriptionTier
	originalTransactionID string
	purchasedAt           time.Time
}

// runVerify verifies every supplied JWS, applies the highest active subscription
// and the highest lifetime purchase to the caller's profile, persists it, and
// syncs Auth0. It never revokes a lifetime grant: the JWS list is
// client-supplied, so a missing transaction is not proof of a refund.
func (h *handler) runVerify(ctx context.Context, userID string, signedTransactions []string) (verifyResponse, error) {
	var resp verifyResponse
	err := profiles.RetryOnConflict(ctx, profiles.DefaultSaveAttempts, func(ctx context.Context) error {
		var err error
		resp, err = h.verifyOnce(ctx, userID, signedTransactions)
		return err
	})
	return resp, err
}

// verifyOnce is one load-modify-save pass of runVerify. Only the profile save
// can report a version conflict, and it runs before any Auth0 or notifier side
// effect, so a conflicting pass leaves nothing to undo.
func (h *handler) verifyOnce(ctx context.Context, userID string, signedTransactions []string) (verifyResponse, error) {
	profile, err := h.profilesByUser.Get(ctx, userID)
	if err != nil {
		if errors.Is(err, profiles.ErrNotFound) {
			return verifyResponse{}, &userNotFoundError{userID: userID}
		}
		return verifyResponse{}, fmt.Errorf("load profile %q: %w", userID, err)
	}
	tierBefore := profile.Tier
	before := subscriptionevents.SnapshotOf(profile)

	now := h.now()
	highestTier := profiles.TierFree
	var highestExpiry time.Time
	var highestOriginalTxn, highestProductID string
	var highestInTrial bool
	var lifetime lifetimeCandidate

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
		if !envAllowed(txn.Environment, h.allowedEnvironments) {
			return verifyResponse{}, &PayloadError{Message: fmt.Sprintf("Transaction environment '%s' is not accepted.", txn.Environment)}
		}
		if !txn.RevocationDate.IsZero() {
			continue
		}
		if txn.Type == TransactionTypeNonConsumable {
			tier, err := TierForProduct(txn.ProductID)
			if err != nil {
				return verifyResponse{}, err
			}
			if tier > lifetime.tier {
				lifetime = lifetimeCandidate{tier: tier, originalTransactionID: txn.OriginalTransactionID, purchasedAt: txn.PurchaseDate}
			}
			continue
		}
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
			highestProductID = txn.ProductID
			highestInTrial = txn.IsFreeTrial()
		}
	}

	if lifetime.tier.IsPaid() {
		if err := h.requireOwnedBy(ctx, userID, lifetime.originalTransactionID); err != nil {
			return verifyResponse{}, err
		}
		profile.GrantLifetime(lifetime.tier, lifetime.originalTransactionID, lifetime.purchasedAt)
	}

	if highestTier == profiles.TierFree {
		profile.ExpireSubscription()
	} else {
		if err := h.requireOwnedBy(ctx, userID, highestOriginalTxn); err != nil {
			return verifyResponse{}, err
		}
		profile.LinkOriginalTransactionID(highestOriginalTxn)
		profile.ActivateAppStoreSubscription(highestTier, highestExpiry, highestProductID, highestInTrial)
	}

	if err := h.profilesByUser.Save(ctx, profile); err != nil {
		if errors.Is(err, profiles.ErrTransactionClaimed) {
			return verifyResponse{}, &conflictError{}
		}
		return verifyResponse{}, fmt.Errorf("save profile %q: %w", userID, err)
	}
	if h.notifier != nil {
		h.notifier.Changed(ctx, before, subscriptionevents.SnapshotOf(profile), subscriptionevents.SourceAppStore)
	}
	if err := h.auth0.UpdateSubscriptionTier(ctx, profile.UserID, profile.Tier.String()); err != nil {
		return verifyResponse{}, fmt.Errorf("sync auth0 tier %q: %w", userID, err)
	}

	effective := profile.EffectiveTier(now)

	// Deliberately logs nothing user-identifying: no user id, JWS content or Apple
	// original transaction id.
	h.logger.InfoContext(ctx, "subscription verify completed",
		"tier", effective.String(),
		"transactionCount", len(signedTransactions),
		"tierChanged", profile.Tier != tierBefore,
	)

	return verifyResponse{
		Tier:               effective.String(),
		SubscriptionExpiry: platform.DotNetTimePtr(profile.SubscriptionExpiry),
		Entitlements:       effective.Entitlements(),
		WatchZoneLimit:     effective.WatchZoneLimit(),
	}, nil
}

// requireOwnedBy enforces single-owner on an Apple original transaction id. A
// signed transaction proves nothing about which account bought it, so without
// this check one JWS could grant Pro on unlimited accounts. It returns a
// *conflictError when a different user already owns the id.
func (h *handler) requireOwnedBy(ctx context.Context, userID, originalTransactionID string) error {
	existing, err := h.profilesByTxn.GetByOriginalTransactionID(ctx, originalTransactionID)
	switch {
	case err != nil && !errors.Is(err, profiles.ErrNotFound):
		return fmt.Errorf("look up transaction owner %q: %w", originalTransactionID, err)
	case err == nil && existing.UserID != userID:
		return &conflictError{}
	}
	return nil
}

// webhook implements POST /v1/webhooks/appstore (App Store Server Notifications
// v2). Apple POSTs lifecycle events here; the call is anonymous and the signed
// JWS is the authentication. Always 200 on success (including duplicates and
// unknown subscribers).
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

	// Cheap structural checks run before the expensive X.509 chain verification
	// so attacker-controlled input cannot drive CPU cost.
	if len(req.SignedPayload) > maxWebhookPayloadBytes || !isCompactJWS(req.SignedPayload) {
		h.malformedBody(r, w)
		return
	}

	if _, err := h.processor.Process(r.Context(), req.SignedPayload); err != nil {
		h.writeWebhookError(r, w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
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
// (a 400), as opposed to a server-side failure (store, Auth0) which is a 500.
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

// apiErrorResponse is the error envelope { error, message }.
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
	body, err := httputil.EncodeJSON(apiErrorResponse{Error: code, Message: &message})
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
