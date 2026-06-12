// Package devicetokens owns the device-token feature: the DeviceRegistration
// domain value, its Cosmos document shape, the Cosmos store, and the
// PUT/DELETE /v1/me/device-token HTTP handlers. It mirrors the .NET
// TownCrier.{Domain,Application,Infrastructure}.DeviceRegistrations slice
// (GH#418 iteration 4) but follows idiomatic Go: a plain struct validated at
// construction, a consumer-side store interface, and hand-written test fakes.
package devicetokens

import (
	"errors"
	"strings"
	"time"
)

// DevicePlatform enumerates the push platforms. The string forms ("Ios",
// "Android") are the exact values the .NET DevicePlatform enum serialises to
// (UseStringEnumConverter) and stores in Cosmos, so they are preserved here.
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
// case-insensitive — the .NET System.Text.Json string-enum converter is
// case-insensitive by default, so "ios" and "Ios" both bind on the inbound side.
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
// (non-blank user id and token), matching .NET DeviceRegistration.Create.
type DeviceRegistration struct {
	UserID       string
	Token        string
	Platform     DevicePlatform
	RegisteredAt time.Time
}

// NewRegistration builds a registration, rejecting a blank user id or token —
// the same guard as .NET's ArgumentException.ThrowIfNullOrWhiteSpace.
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

// Refresh stamps RegisteredAt to now unconditionally, mirroring .NET
// RefreshRegistration: a re-PUT records the client's instant even when it is
// earlier than the stored one (the client's clock is authoritative, and the
// re-write resets the Cosmos TTL).
func (r *DeviceRegistration) Refresh(now time.Time) {
	r.RegisteredAt = now
}
