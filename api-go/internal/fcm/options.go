package fcm

import "fmt"

// firebaseMessagingScope is the OAuth2 scope a service-account access token must
// carry to call the FCM HTTP v1 send endpoint.
const firebaseMessagingScope = "https://www.googleapis.com/auth/firebase.messaging"

// productionBaseURL is the FCM HTTP v1 host. The per-project send path
// (/v1/projects/{id}/messages:send) is appended by the client; the host is a
// well-known Google endpoint, not configurable infrastructure.
const productionBaseURL = "https://fcm.googleapis.com"

// Options configures the FCM sender. When Enabled is false the caller wires a
// NoOpSender so a worker without a service-account key boots cleanly. When true,
// ProjectID and ServiceAccountJSON must both be populated.
type Options struct {
	// Enabled gates whether a real sender is constructed.
	Enabled bool
	// ProjectID is the Firebase/GCP project id the send URL targets
	// (/v1/projects/{ProjectID}/messages:send).
	ProjectID string
	// ServiceAccountJSON is the full service-account key JSON blob (the same
	// document Firebase issues), carrying the RSA private key, client email, and
	// token URI the JWT-bearer OAuth exchange uses.
	ServiceAccountJSON string
}

// validate checks the required fields when the sender is enabled. It is a no-op
// when disabled so a worker without FCM config boots without the fields.
func (o Options) validate() error {
	if !o.Enabled {
		return nil
	}
	if o.ProjectID == "" {
		return fmt.Errorf("fcm: %w: ProjectID is empty", ErrInvalidOptions)
	}
	if o.ServiceAccountJSON == "" {
		return fmt.Errorf("fcm: %w: ServiceAccountJSON is empty", ErrInvalidOptions)
	}
	return nil
}
