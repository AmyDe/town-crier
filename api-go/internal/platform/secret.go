package platform

import "log/slog"

// SecretString wraps a credential so it cannot be accidentally logged or
// serialised. String, MarshalJSON, and LogValue all redact; the raw value is
// reachable only via Expose, called at the single boundary where the secret
// leaves the process (e.g. an Authorization header). Use it for the Auth0 M2M
// client secret and any other credential threaded through config.
type SecretString struct {
	value string
}

// NewSecret wraps a raw credential string.
func NewSecret(v string) SecretString { return SecretString{value: v} }

// String redacts the secret so it is safe in fmt and log output.
func (s SecretString) String() string { return "[REDACTED]" }

// MarshalJSON redacts the secret so it never appears in serialised config.
func (s SecretString) MarshalJSON() ([]byte, error) { return []byte(`"[REDACTED]"`), nil }

// LogValue redacts the secret for log/slog, which otherwise inspects the value
// directly rather than going through String.
func (s SecretString) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }

// Expose returns the raw credential. Call only where the secret must leave the
// process; never log or serialise the result.
func (s SecretString) Expose() string { return s.value }
