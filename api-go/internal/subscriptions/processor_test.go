package subscriptions

import (
	"context"
	"log/slog"
	"testing"

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
	d := &processorTestDeps{
		verifier:    &fakeVerifier{results: map[string]string{}, errs: map[string]error{}},
		byTxn:       &fakeProfileByTxn{},
		auth0:       &fakeAuth0{},
		idempotency: newFakeIdempotency(),
	}
	d.processor = NewNotificationProcessor(d.verifier, d.byTxn, d.auth0, d.idempotency, allowedEnvs, slog.New(slog.DiscardHandler))
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
