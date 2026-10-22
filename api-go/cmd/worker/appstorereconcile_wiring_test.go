package main

import (
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// validAppStoreServerAPIKeyPEM is a syntactically valid PKCS8 EC private key,
// not a real Apple-issued key.
const validAppStoreServerAPIKeyPEM = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQg1KGB5FBmTOozZK3o
7Q7VYVZEeCB6iF6dO1e3q9pS0PmhRANCAATeZWc+Mrg+K5c5g8Qk3P5Q4uNTBP1B
xNJnfnRc+/L0iAaJ/rMYuI9O+M8fJ2WRPBQIxHUXMz9WjZ54zVIYD4RH
-----END PRIVATE KEY-----`

func TestBuildAppStoreReconcile_UnconfiguredReturnsNil(t *testing.T) {
	t.Parallel()

	handler := buildAppStoreReconcile(platform.Config{}, &stores{}, discardLogger())

	if handler != nil {
		t.Fatalf("buildAppStoreReconcile: got non-nil handler, want nil (APPSTORE_RECONCILE_ENABLED unset)")
	}
}

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
