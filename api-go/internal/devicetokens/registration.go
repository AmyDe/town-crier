// Package devicetokens owns the device-token feature: the DeviceRegistration
// domain value, its Cosmos document shape, the Cosmos store, and the
// PUT/DELETE /v1/me/device-token HTTP handlers (GH#418). A plain struct
// validated at construction, a consumer-side store interface, and hand-written
// test fakes.
package devicetokens

import (
	"errors"
	"strings"
	"time"
)

// DevicePlatform enumerates the push platforms. The string forms ("Ios",
// "Android") are the exact values stored in Cosmos and sent over the wire,
// so they are preserved here.
type DevicePlatform int

const (
	// PlatformIos is Apple Push Notification service.
	PlatformIos DevicePlatform = iota
	// PlatformAndroid is Firebase Cloud Messaging.
	PlatformAndroid
)

// String returns the canonical wire/storage form of the platform.
func (p DevicePlatform) String() string {
	switch p {
	case PlatformIos:
		return "Ios"
	case PlatformAndroid:
		return "Android"
	default:
		return "Ios"
	}
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

// Refresh stamps RegisteredAt to now unconditionally: a re-PUT records the
// client's instant even when it is earlier than the stored one (the client's
// clock is authoritative, and the re-write resets the Cosmos TTL).
func (r *DeviceRegistration) Refresh(now time.Time) {
	r.RegisteredAt = now
}
