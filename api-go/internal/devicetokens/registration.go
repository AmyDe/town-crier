// Package devicetokens owns device push registrations: the DeviceRegistration
// value, its Postgres store, and the PUT/DELETE /v1/me/device-token handlers.
package devicetokens

import (
	"errors"
	"strings"
	"time"
)

// DevicePlatform enumerates the push platforms. The string forms ("Ios",
// "Android") are the exact stored and wire values.
type DevicePlatform int

const (
	// PlatformIos is Apple Push Notification service.
	PlatformIos DevicePlatform = iota
	// PlatformAndroid is Firebase Cloud Messaging.
	PlatformAndroid
)

// String returns the canonical wire/storage form of the platform.
func (p DevicePlatform) String() string {
	if p == PlatformAndroid {
		return "Android"
	}
	return "Ios"
}

// ErrUnknownPlatform is returned by ParsePlatform for an unrecognised value.
var ErrUnknownPlatform = errors.New("unknown device platform")

// ParsePlatform converts a wire/stored platform string to the enum. The match is
// case-insensitive, so "ios" and "Ios" both bind on the inbound side.
func ParsePlatform(s string) (DevicePlatform, error) {
	switch {
	case strings.EqualFold(s, "Ios"):
		return PlatformIos, nil
	case strings.EqualFold(s, "Android"):
		return PlatformAndroid, nil
	default:
		return 0, ErrUnknownPlatform
	}
}

// DeviceRegistration is one (user, token) push registration. Exported fields
// keep it a plain Go value; the constructor enforces the only real invariants
// (non-blank user id and token).
type DeviceRegistration struct {
	UserID       string
	Token        string
	Platform     DevicePlatform
	RegisteredAt time.Time
}

// NewRegistration builds a registration, rejecting a blank user id or token —
// rejects a blank (whitespace-only) user id or token.
func NewRegistration(userID, token string, platform DevicePlatform, now time.Time) (DeviceRegistration, error) {
	if strings.TrimSpace(userID) == "" {
		return DeviceRegistration{}, errors.New("user id is required")
	}
	if strings.TrimSpace(token) == "" {
		return DeviceRegistration{}, errors.New("token is required")
	}
	return DeviceRegistration{
		UserID:       userID,
		Token:        token,
		Platform:     platform,
		RegisteredAt: now,
	}, nil
}

// Refresh stamps RegisteredAt to now unconditionally, even when it is earlier
// than the stored instant: the client's clock is authoritative.
func (r *DeviceRegistration) Refresh(now time.Time) {
	r.RegisteredAt = now
}
