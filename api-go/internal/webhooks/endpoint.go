package webhooks

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// Format is the body shape a webhook receives. The values are stored and served.
type Format string

// Format values.
const (
	FormatJSON    Format = "json"
	FormatSlack   Format = "slack"
	FormatDiscord Format = "discord"
)

// ErrInvalidFormat reports a format other than json, slack or discord.
var ErrInvalidFormat = errors.New("webhooks: invalid format")

// ParseFormat returns the Format for its wire value, or ErrInvalidFormat.
func ParseFormat(s string) (Format, error) {
	switch f := Format(s); f {
	case FormatJSON, FormatSlack, FormatDiscord:
		return f, nil
	default:
		return "", ErrInvalidFormat
	}
}

// Status says whether alerts are queued for an endpoint.
type Status string

// Status values.
const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// DisabledReason says why an endpoint was switched off.
type DisabledReason string

// DisabledReason values.
const (
	DisabledFailures DisabledReason = "failures"
	DisabledGone     DisabledReason = "gone"
)

const (
	// MaxConsecutiveFailures is the number of consecutive failed runs that
	// switches an endpoint off.
	MaxConsecutiveFailures = 24
	// MaxDeliveryAttempts is the number of failed attempts after which a
	// delivery is given up.
	MaxDeliveryAttempts = 6
	// TestCooldown is the minimum gap between two test events for an endpoint.
	TestCooldown = 10 * time.Second
)

// SwitchOffReason applies the switch-off rules after a failed run: a gone
// endpoint is switched off at once, any other once consecutiveFailures (which
// includes this run) reaches MaxConsecutiveFailures.
func SwitchOffReason(failure FailureReason, consecutiveFailures int) (DisabledReason, bool) {
	switch {
	case failure == FailureGone:
		return DisabledGone, true
	case consecutiveFailures >= MaxConsecutiveFailures:
		return DisabledFailures, true
	default:
		return "", false
	}
}

// Endpoint is a user's single webhook.
type Endpoint struct {
	ID     string
	UserID string
	URL    string
	Format Format
	// Secret is the signing key. It is shown to the user only when it is
	// created or rotated.
	Secret              string
	Status              Status
	DisabledReason      *DisabledReason
	ConsecutiveFailures int
	LastSuccessAt       *time.Time
	LastFailureAt       *time.Time
	LastFailureReason   *FailureReason
	LastFailureStatus   *int
	LastTestAt          *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// LogValue limits a logged Endpoint to its id.
func (e Endpoint) LogValue() slog.Value {
	return slog.GroupValue(slog.String("id", e.ID))
}

// NewEndpoint builds an active endpoint with a fresh secret. It returns
// ErrInvalidURL or ErrInvalidFormat for input that must not be saved.
func NewEndpoint(id, userID, rawURL string, format Format, now time.Time) (Endpoint, error) {
	if err := validateConfig(rawURL, format); err != nil {
		return Endpoint{}, err
	}
	return Endpoint{
		ID:        id,
		UserID:    userID,
		URL:       rawURL,
		Format:    format,
		Secret:    NewSecret(),
		Status:    StatusActive,
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
	}, nil
}

func validateConfig(rawURL string, format Format) error {
	if err := ValidateURL(rawURL); err != nil {
		return err
	}
	if _, err := ParseFormat(string(format)); err != nil {
		return fmt.Errorf("validate format: %w", err)
	}
	return nil
}

const (
	secretPrefix = "tcwh_"
	secretBytes  = 32
)

// NewSecret returns a new signing secret: "tcwh_" followed by 32 random bytes
// in unpadded base64url.
func NewSecret() string {
	var b [secretBytes]byte
	rand.Read(b[:]) // never fails since Go 1.24; an unrecoverable failure crashes the process
	return secretPrefix + base64.RawURLEncoding.EncodeToString(b[:])
}
