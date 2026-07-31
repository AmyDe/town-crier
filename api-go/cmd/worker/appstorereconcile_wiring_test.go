package main

import (
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// validAppStoreServerAPIKeyPEM is a syntactically valid PKCS8 EC private key,
// generated once for this test file — not a real Apple-issued key. It only
// needs to parse; appstorereconcile's Handler never dials Apple in this test.
const validAppStoreServerAPIKeyPEM = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg1KGB5FBmTOozZK3o
7Q7VYVZEeCB6iF6dO1e3q9pS0PmhRANCAATeZWc+Mrg+K5c5g8Qk3P5Q4uNTBP1B
xNJnfnRc+/L0iAaJ/rMYuI9O+M8fJ2WRPBQIxHUXMz9WjZ54zVIYD4RH
-----END PRIVATE KEY-----`

// TestBuildAppStoreReconcile_UnconfiguredReturnsNil proves buildAppStoreReconcile
// returns a genuine nil *appstorereconcile.Handler — not just a nil-checked
// error — whenever APPSTORE_RECONCILE_ENABLED is unset, mirroring the
// "unconfigured optional job" posture buildDevSeeder/buildPollOrchestrator
// already use.
func TestBuildAppStoreReconcile_UnconfiguredReturnsNil(t *testing.T) {
	t.Parallel()

	handler := buildAppStoreReconcile(platform.Config{}, &stores{}, discardLogger())

	if handler != nil {
		t.Fatalf("buildAppStoreReconcile: got non-nil handler, want nil (APPSTORE_RECONCILE_ENABLED unset)")
	}
}

// TestBuildAppStoreReconcile_MalformedKeyReturnsNil proves an enabled job with
// key material that fails to parse also returns nil rather than panicking or
// propagating a fatal error — a malformed .p8 must not crash-loop the other
// modes (digest, dormant-cleanup, etc.) this same binary dispatches.
func TestBuildAppStoreReconcile_MalformedKeyReturnsNil(t *testing.T) {
	t.Parallel()
	cfg := platform.Config{
		AppStoreReconcileEnabled:     true,
		AppStoreServerAPIKey:         platform.NewSecret("not a pem"),
		AppStoreServerAPIKeyID:       "ABCDE12345",
		AppStoreServerAPIIssuerID:    "57246542-96fe-1a63-e053-0824d011072a",
		AppStoreServerAPIEnvironment: "Sandbox",
		AppleBundleID:                "uk.towncrierapp.mobile",
		AppleEnvironments:            []string{"Production"},
	}

	handler := buildAppStoreReconcile(cfg, &stores{}, discardLogger())

	if handler != nil {
		t.Fatalf("buildAppStoreReconcile: got non-nil handler, want nil (malformed signing key)")
	}
}

// TestBuildAppStoreReconcile_ConfiguredReturnsNonNilHandler proves
// buildAppStoreReconcile builds a real Handler once enabled with valid key
// material. It needs no live Apple/Postgres access: appstoreserverapi.NewClient
// and subscriptions.NewJWSVerifier both construct lazily — no network call
// happens until Handler.Run is invoked.
func TestBuildAppStoreReconcile_ConfiguredReturnsNonNilHandler(t *testing.T) {
	t.Parallel()
	cfg := platform.Config{
		AppStoreReconcileEnabled:       true,
		AppStoreServerAPIKey:           platform.NewSecret(validAppStoreServerAPIKeyPEM),
		AppStoreServerAPIKeyID:         "ABCDE12345",
		AppStoreServerAPIIssuerID:      "57246542-96fe-1a63-e053-0824d011072a",
		AppStoreServerAPIEnvironment:   "Sandbox",
		AppStoreReconcileApplyEnabled:  false,
		AppStoreReconcileLookbackHours: 30,
		AppleBundleID:                  "uk.towncrierapp.mobile",
		AppleEnvironments:              []string{"Production"},
	}

	handler := buildAppStoreReconcile(cfg, &stores{}, discardLogger())

	if handler == nil {
		t.Fatal("buildAppStoreReconcile: got nil handler, want a configured Handler")
	}
}
