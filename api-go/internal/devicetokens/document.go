package devicetokens

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// deviceTTLSeconds is the Cosmos document TTL: device registrations that stop
// refreshing (app uninstalled, logged out) are purged after 180 days so the
// push-token store doesn't accumulate stale records. Every PUT re-upserts and
// resets _ts. Mirrors .NET DeviceRegistrationDocument's 180-day TTL constant
// (UK GDPR Art. 5(1)(e) storage limitation for device identifiers).
const deviceTTLSeconds = 180 * 24 * 60 * 60

// deviceDocument is the Cosmos persistence shape for a DeviceRegistration. The
// JSON tags reproduce the camelCase keys the .NET CosmosDeviceRegistrationRepository
// writes, so a Go-written document is byte-compatible with the existing
// DeviceRegistrations container and an existing document hydrates here unchanged.
//
// Partition key: the container is partitioned by /userId and the document id is
// the token, so every operation is a single-partition point operation keyed on
// (userId, token).
type deviceDocument struct {
	ID           string              `json:"id"`
	UserID       string              `json:"userId"`
	Token        string              `json:"token"`
	Platform     string              `json:"platform"`
	RegisteredAt platform.DotNetTime `json:"registeredAt"`
	TTL          int                 `json:"ttl"`
}

// newDeviceDocument maps a domain registration to its persistence shape. The
// document id equals the token (the .NET FromDomain mapping); the partition key
// is the user id, supplied separately to the store's upsert.
func newDeviceDocument(r DeviceRegistration) deviceDocument {
	return deviceDocument{
		ID:           r.Token,
		UserID:       r.UserID,
		Token:        r.Token,
		Platform:     r.Platform.String(),
		RegisteredAt: platform.DotNetTime(r.RegisteredAt),
		TTL:          deviceTTLSeconds,
	}
}

// toDomain reconstitutes a domain registration from its stored document,
// parsing the string platform (an unknown value is an error, matching .NET's
// Enum.Parse).
func (d deviceDocument) toDomain() (DeviceRegistration, error) {
	platformValue, err := ParsePlatform(d.Platform)
	if err != nil {
		return DeviceRegistration{}, err
	}
	return DeviceRegistration{
		UserID:       d.UserID,
		Token:        d.Token,
		Platform:     platformValue,
		RegisteredAt: time.Time(d.RegisteredAt),
	}, nil
}
