package subscriptions

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// newTestProcessor builds a NotificationProcessor over freshly minted fakes for
// direct (non-HTTP) Process calls, mirroring newTestDepsWithEnvsAndLogger's
// wiring so the two test surfaces (HTTP handler_test.go, direct
// processor_test.go) exercise identical collaborator behaviour.
type processorTestDeps struct {
	verifier    *fakeVerifier
	byTxn       *fakeProfileByTxn
	auth0       *fakeAuth0
	idempotency *fakeIdempotency
	processor   *NotificationProcessor
}

func newTestProcessor(allowedEnvs []string) *processorTestDeps {
	return newTestProcessorWithLogger(allowedEnvs, slog.New(slog.DiscardHandler))
}

// newTestProcessorWithLogger builds a processor with a caller-supplied logger,
// so a test can assert on emitted log lines (mirrors handler_test.go's
// newTestDepsWithEnvsAndLogger).
func newTestProcessorWithLogger(allowedEnvs []string, logger *slog.Logger) *processorTestDeps {
	d := &processorTestDeps{
		verifier:    &fakeVerifier{results: map[string]string{}, errs: map[string]error{}},
		byTxn:       &fakeProfileByTxn{},
		auth0:       &fakeAuth0{},
		idempotency: newFakeIdempotency(),
	}
	d.processor = NewNotificationProcessor(d.verifier, d.byTxn, d.auth0, d.idempotency, allowedEnvs, logger)
	return d
}

// TestNotificationProcessor_Process_SubscribedActivatesProfile pins wasNew=true
// for a SUBSCRIBED notification against a known subscriber — the byte-identical
// behaviour of the former runWebhook, extended with the new return value.
func TestNotificationProcessor_Process_SubscribedActivatesProfile(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = freshProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("SUBSCRIBED", "uuid-1", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	wasNew, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !wasNew {
		t.Error("wasNew = false, want true (profile activated)")
	}
	if d.byTxn.saved == nil || d.byTxn.saved.Tier != profiles.TierPro {
		t.Error("profile not activated to Pro")
	}
	if len(d.auth0.tiers) != 1 || d.auth0.tiers[0] != "Pro" {
		t.Errorf("auth0 sync = %v, want [Pro]", d.auth0.tiers)
	}
	if len(d.idempotency.marked) != 1 || d.idempotency.marked[0] != "uuid-1" {
		t.Errorf("marked = %v, want [uuid-1]", d.idempotency.marked)
	}
}

// TestNotificationProcessor_Process_DidRenewIsWasNewTrue pins wasNew=true for a
// DID_RENEW event against a known subscriber.
func TestNotificationProcessor_Process_DidRenewIsWasNewTrue(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = freshProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("DID_RENEW", "uuid-renew", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	wasNew, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !wasNew {
		t.Error("wasNew = false, want true (renewal applied)")
	}
}

// TestNotificationProcessor_Process_UpgradeIsWasNewTrue pins wasNew=true for a
// DID_CHANGE_RENEWAL_PREF/UPGRADE event.
func TestNotificationProcessor_Process_UpgradeIsWasNewTrue(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = freshProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSONWithSubtype("DID_CHANGE_RENEWAL_PREF", "UPGRADE", "uuid-up", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	wasNew, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !wasNew {
		t.Error("wasNew = false, want true (upgrade applied)")
	}
}

// TestNotificationProcessor_Process_DowngradeIsWasNewFalse pins wasNew=false for
// a DID_CHANGE_RENEWAL_PREF/DOWNGRADE event — a no-op that takes effect at the
// next renewal, so no state actually changed.
func TestNotificationProcessor_Process_DowngradeIsWasNewFalse(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = freshProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSONWithSubtype("DID_CHANGE_RENEWAL_PREF", "DOWNGRADE", "uuid-down", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	wasNew, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if wasNew {
		t.Error("wasNew = true, want false (downgrade is a no-op until next renewal)")
	}
	if d.byTxn.saved != nil {
		t.Error("downgrade must not save the profile")
	}
	// A no-change event is still marked processed so Apple does not retry.
	if len(d.idempotency.marked) != 1 {
		t.Errorf("marked = %v, want exactly one entry", d.idempotency.marked)
	}
}

// TestNotificationProcessor_Process_DuplicateIsWasNewFalse pins wasNew=false
// for an already-processed notification (dedup).
func TestNotificationProcessor_Process_DuplicateIsWasNewFalse(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = freshProfile(t)
	d.idempotency.processed["uuid-dup"] = true
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("SUBSCRIBED", "uuid-dup", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	wasNew, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if wasNew {
		t.Error("wasNew = true, want false (duplicate)")
	}
	if d.byTxn.saved != nil {
		t.Error("duplicate should not save the profile")
	}
	if len(d.idempotency.marked) != 0 {
		t.Errorf("duplicate should not re-mark, got %v", d.idempotency.marked)
	}
}

// TestNotificationProcessor_Process_UnknownSubscriberIsWasNewFalse pins
// wasNew=false when no profile owns the transaction's original transaction id.
func TestNotificationProcessor_Process_UnknownSubscriberIsWasNewFalse(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = nil // no matching subscriber
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("DID_RENEW", "uuid-unknown", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-x", futureExpiryMs())

	wasNew, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if wasNew {
		t.Error("wasNew = true, want false (unknown subscriber)")
	}
	if len(d.idempotency.marked) != 1 {
		t.Errorf("should still mark processed, got %v", d.idempotency.marked)
	}
}

// TestNotificationProcessor_Process_EnvironmentMismatchIsWasNewFalse pins
// wasNew=false when the inner transaction's environment is outside the
// allowlist.
func TestNotificationProcessor_Process_EnvironmentMismatchIsWasNewFalse(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs) // allowedEnvs = ["Production"]
	d.byTxn.profile = freshProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("SUBSCRIBED", "uuid-env", "INNER_SBX")
	d.verifier.results["INNER_SBX"] = txnJSONEnv(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs(), "Sandbox")

	wasNew, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if wasNew {
		t.Error("wasNew = true, want false (environment mismatch)")
	}
	if d.byTxn.saved != nil {
		t.Error("profile must not be mutated on environment mismatch")
	}
	if len(d.auth0.tiers) != 0 {
		t.Error("auth0 must not be synced on environment mismatch")
	}
	if len(d.idempotency.marked) != 1 || d.idempotency.marked[0] != "uuid-env" {
		t.Errorf("should still mark processed, got %v", d.idempotency.marked)
	}
}

// TestNotificationProcessor_Process_VerifierErrorPropagates asserts a JWS
// verification failure surfaces as an error with wasNew=false.
func TestNotificationProcessor_Process_VerifierErrorPropagates(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.verifier.errs["hdr.BAD.sig"] = &JWSVerificationError{Message: "bad chain"}

	wasNew, err := d.processor.Process(context.Background(), "hdr.BAD.sig")

	if err == nil {
		t.Fatal("Process: want error, got nil")
	}
	if wasNew {
		t.Error("wasNew = true, want false on error")
	}
}

// TestNotificationProcessor_Process_Auth0UserNotFoundIsNotFatal pins the
// GH#1165 Phase 1 contract: a missing Auth0 user during the tier sync does not
// fail the notification, because the Postgres profile is already saved and is
// the source of truth.
func TestNotificationProcessor_Process_Auth0UserNotFoundIsNotFatal(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = freshProfile(t)
	d.auth0.err = profiles.ErrAuth0UserNotFound
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("SUBSCRIBED", "uuid-404", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	wasNew, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err != nil {
		t.Fatalf("Process: %v, want nil (missing Auth0 user is not fatal)", err)
	}
	if !wasNew {
		t.Error("wasNew = false, want true (profile still activated)")
	}
	if d.byTxn.saved == nil || d.byTxn.saved.Tier != profiles.TierPro {
		t.Error("profile not saved despite missing Auth0 user")
	}
	if len(d.idempotency.marked) != 1 || d.idempotency.marked[0] != "uuid-404" {
		t.Errorf("marked = %v, want [uuid-404]", d.idempotency.marked)
	}
}

// TestNotificationProcessor_Process_Auth0GenericErrorFails asserts that a
// non-404 Auth0 failure still fails the notification, so Apple retries a real
// outage.
func TestNotificationProcessor_Process_Auth0GenericErrorFails(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = freshProfile(t)
	d.auth0.err = errors.New("auth0 unreachable")
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("SUBSCRIBED", "uuid-500", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-1", futureExpiryMs())

	_, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err == nil {
		t.Fatal("Process: want error, got nil")
	}
	if len(d.idempotency.marked) != 0 {
		t.Errorf("must not mark processed on a real Auth0 failure, got %v", d.idempotency.marked)
	}
}

// TestNotificationProcessor_Process_Auth0UserNotFoundLogsWarnWithoutIdentifiers
// asserts the Warn log carries no user id, notification uuid, transaction id
// or JWS content.
func TestNotificationProcessor_Process_Auth0UserNotFoundLogsWarnWithoutIdentifiers(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	d := newTestProcessorWithLogger(testAllowedEnvs, logger)
	d.byTxn.profile = freshProfile(t)
	d.auth0.err = profiles.ErrAuth0UserNotFound
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("SUBSCRIBED", "uuid-warn", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-warn", futureExpiryMs())

	if _, err := d.processor.Process(context.Background(), "hdr.OUTER.sig"); err != nil {
		t.Fatalf("Process: %v", err)
	}

	line, ok := findLogLine(decodeJSONLogLines(t, buf.Bytes()), "auth0 user missing for subscription profile")
	if !ok {
		t.Fatalf("expected warn log not found in log output: %s", buf.String())
	}
	if line["level"] != "WARN" {
		t.Errorf("level = %v, want WARN", line["level"])
	}
	for _, forbidden := range []string{"userID", "user", "userId", "notificationUUID", "originalTransactionId", "jws", "signedPayload"} {
		if _, present := line[forbidden]; present {
			t.Errorf("log line must not contain %q, got %v", forbidden, line[forbidden])
		}
	}
	if strings.Contains(buf.String(), testUserID) {
		t.Errorf("log output must not contain the userID %q: %s", testUserID, buf.String())
	}
	if strings.Contains(buf.String(), "uuid-warn") {
		t.Errorf("log output must not contain the notification uuid: %s", buf.String())
	}
	if strings.Contains(buf.String(), "orig-warn") {
		t.Errorf("log output must not contain the Apple original transaction id: %s", buf.String())
	}
}

func lifetimeProfile(t *testing.T) *profiles.UserProfile {
	t.Helper()
	p := freshProfile(t)
	p.GrantLifetime(profiles.TierPro, "life-1", testNow.AddDate(0, -1, 0))
	return p
}

func TestNotificationProcessor_Process_ExpiredMonthlyKeepsLifetime(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	p := lifetimeProfile(t)
	p.LinkOriginalTransactionID("orig-sub")
	p.ActivateAppStoreSubscription(profiles.TierPro, testNow.AddDate(0, 0, -1), ProductProMonthly)
	d.byTxn.profile = p
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("EXPIRED", "uuid-exp", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProMonthly, testBundleID, "orig-sub", pastExpiryMs())

	wasNew, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !wasNew {
		t.Error("wasNew = false, want true")
	}
	saved := d.byTxn.saved
	if saved == nil {
		t.Fatal("profile not saved")
	}
	if saved.Tier != profiles.TierPro {
		t.Errorf("Tier = %v, want Pro", saved.Tier)
	}
	if got := saved.EffectiveTier(testNow); got != profiles.TierPro {
		t.Errorf("EffectiveTier = %v, want Pro", got)
	}
	if len(d.auth0.tiers) != 1 || d.auth0.tiers[0] != "Pro" {
		t.Errorf("auth0 sync = %v, want [Pro]", d.auth0.tiers)
	}
}

func TestNotificationProcessor_Process_RefundLifetimeRevokes(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = lifetimeProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("REFUND", "uuid-refund", "INNER")
	d.verifier.results["INNER"] = lifetimeTxnJSON(ProductProLifetime, "life-1", lifetimePurchaseMs+1000)

	wasNew, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !wasNew {
		t.Error("wasNew = false, want true")
	}
	requireLifetimeRevoked(t, d)
}

func TestNotificationProcessor_Process_RevokeLifetimeRevokes(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = lifetimeProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("REVOKE", "uuid-revoke", "INNER")
	d.verifier.results["INNER"] = lifetimeTxnJSON(ProductProLifetime, "life-1", lifetimePurchaseMs+1000)

	if _, err := d.processor.Process(context.Background(), "hdr.OUTER.sig"); err != nil {
		t.Fatalf("Process: %v", err)
	}

	requireLifetimeRevoked(t, d)
}

func requireLifetimeRevoked(t *testing.T, d *processorTestDeps) {
	t.Helper()
	saved := d.byTxn.saved
	if saved == nil {
		t.Fatal("profile not saved")
	}
	if saved.Tier != profiles.TierFree || saved.LifetimeTier != profiles.TierFree {
		t.Errorf("Tier/LifetimeTier = %v/%v, want Free/Free", saved.Tier, saved.LifetimeTier)
	}
	if saved.LifetimeOriginalTransactionID != nil || saved.LifetimePurchasedAt != nil {
		t.Errorf("lifetime fields = %v/%v, want nil", saved.LifetimeOriginalTransactionID, saved.LifetimePurchasedAt)
	}
	if len(d.auth0.tiers) != 1 || d.auth0.tiers[0] != "Free" {
		t.Errorf("auth0 sync = %v, want [Free]", d.auth0.tiers)
	}
}

func TestNotificationProcessor_Process_OneTimeChargeGrantsLifetime(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = freshProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("ONE_TIME_CHARGE", "uuid-otc", "INNER")
	d.verifier.results["INNER"] = lifetimeTxnJSON(ProductProLifetime, "life-1", 0)

	wasNew, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !wasNew {
		t.Error("wasNew = false, want true")
	}
	saved := d.byTxn.saved
	if saved == nil {
		t.Fatal("profile not saved")
	}
	if saved.Tier != profiles.TierPro || saved.LifetimeTier != profiles.TierPro {
		t.Errorf("Tier/LifetimeTier = %v/%v, want Pro/Pro", saved.Tier, saved.LifetimeTier)
	}
	if saved.LifetimeOriginalTransactionID == nil || *saved.LifetimeOriginalTransactionID != "life-1" {
		t.Errorf("LifetimeOriginalTransactionID = %v, want life-1", saved.LifetimeOriginalTransactionID)
	}
	if want := time.UnixMilli(lifetimePurchaseMs).UTC(); saved.LifetimePurchasedAt == nil || !saved.LifetimePurchasedAt.Equal(want) {
		t.Errorf("LifetimePurchasedAt = %v, want %v", saved.LifetimePurchasedAt, want)
	}
	if len(d.auth0.tiers) != 1 || d.auth0.tiers[0] != "Pro" {
		t.Errorf("auth0 sync = %v, want [Pro]", d.auth0.tiers)
	}
}

func TestNotificationProcessor_Process_RefundReversedRestoresLifetime(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	p := lifetimeProfile(t)
	p.RevokeLifetime()
	d.byTxn.profile = p
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("REFUND_REVERSED", "uuid-rr", "INNER")
	d.verifier.results["INNER"] = lifetimeTxnJSON(ProductProLifetime, "life-1", 0)

	wasNew, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if !wasNew {
		t.Error("wasNew = false, want true")
	}
	saved := d.byTxn.saved
	if saved == nil || saved.Tier != profiles.TierPro || saved.LifetimeTier != profiles.TierPro {
		t.Errorf("saved = %+v, want lifetime Pro restored", saved)
	}
}

func TestNotificationProcessor_Process_LifetimeUnrelatedNotificationIsNoChange(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = lifetimeProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("EXPIRED", "uuid-x", "INNER")
	d.verifier.results["INNER"] = lifetimeTxnJSON(ProductProLifetime, "life-1", 0)

	wasNew, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if wasNew || d.byTxn.saved != nil {
		t.Errorf("wasNew=%v saved=%v, want no change for a non-consumable EXPIRED", wasNew, d.byTxn.saved)
	}
	if len(d.idempotency.marked) != 1 {
		t.Errorf("marked = %v, want exactly one entry", d.idempotency.marked)
	}
}

func TestNotificationProcessor_Process_OneTimeChargeUnknownProductErrors(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = freshProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("ONE_TIME_CHARGE", "uuid-unk", "INNER")
	d.verifier.results["INNER"] = lifetimeTxnJSON("uk.towncrierapp.mystery.lifetime", "life-1", 0)

	_, err := d.processor.Process(context.Background(), "hdr.OUTER.sig")

	var upe *UnknownProductError
	if !errors.As(err, &upe) {
		t.Fatalf("Process err = %v, want *UnknownProductError", err)
	}
	if d.byTxn.saved != nil {
		t.Error("profile must not be saved for an unknown product")
	}
}

func TestNotificationProcessor_Process_DidRenewStoresProductID(t *testing.T) {
	t.Parallel()
	d := newTestProcessor(testAllowedEnvs)
	d.byTxn.profile = freshProfile(t)
	d.verifier.results["hdr.OUTER.sig"] = notificationJSON("DID_RENEW", "uuid-renew-annual", "INNER")
	d.verifier.results["INNER"] = txnJSON(ProductProAnnual, testBundleID, "orig-1", futureExpiryMs())

	if _, err := d.processor.Process(context.Background(), "hdr.OUTER.sig"); err != nil {
		t.Fatalf("Process: %v", err)
	}

	saved := d.byTxn.saved
	if saved == nil || saved.SubscriptionProductID == nil || *saved.SubscriptionProductID != ProductProAnnual {
		t.Errorf("saved = %+v, want SubscriptionProductID %s", saved, ProductProAnnual)
	}
}
