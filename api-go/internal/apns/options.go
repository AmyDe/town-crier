package apns

import "fmt"

// APNs publishes two fixed endpoints: production and sandbox. They are
// well-known Apple URLs, not configurable infrastructure.
const (
	productionURL = "https://api.push.apple.com"
	sandboxURL    = "https://api.sandbox.push.apple.com"
)

// appleIDLength is the fixed length Apple issues both Key IDs and Team IDs at;
// any other length is a misconfiguration.
const appleIDLength = 10

// Options configures the APNs sender. When Enabled is false the caller wires a
// NoOpSender so a missing .p8 auth key is not fatal in local dev. When true,
// AuthKey/KeyID/TeamID/BundleID must all be populated.
type Options struct {
	// Enabled gates whether a real sender is constructed.
	Enabled bool
	// AuthKey is the PEM contents of the .p8 APNs auth key issued by Apple.
	AuthKey string
	// KeyID is the 10-character Apple Key ID carried in the JWT header (kid).
	KeyID string
	// TeamID is the 10-character Apple Team ID carried in the JWT payload (iss).
	TeamID string
	// BundleID is the iOS app's bundle identifier, sent as the apns-topic header.
	BundleID string
	// UseSandbox routes to the sandbox endpoint (TestFlight / development builds)
	// rather than production (App Store builds).
	UseSandbox bool
}

// baseURL resolves the APNs endpoint for the configured environment.
func (o Options) baseURL() string {
	if o.UseSandbox {
		return sandboxURL
	}
	return productionURL
}

// validate checks the auth fields when the sender is enabled. It is a no-op when
// disabled so local dev hosts boot without auth fields.
func (o Options) validate() error {
	if !o.Enabled {
		return nil
	}
	if o.AuthKey == "" {
		return fmt.Errorf("apns: %w: AuthKey is empty", ErrInvalidOptions)
	}
	if len(o.KeyID) != appleIDLength {
		return fmt.Errorf("apns: %w: KeyID must be %d characters, got %d", ErrInvalidOptions, appleIDLength, len(o.KeyID))
	}
	if len(o.TeamID) != appleIDLength {
		return fmt.Errorf("apns: %w: TeamID must be %d characters, got %d", ErrInvalidOptions, appleIDLength, len(o.TeamID))
	}
	if o.BundleID == "" {
		return fmt.Errorf("apns: %w: BundleID is empty", ErrInvalidOptions)
	}
	return nil
}
