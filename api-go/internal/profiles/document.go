package profiles

import (
	"time"
)

// profileDocument is the Cosmos persistence shape for a UserProfile. The JSON
// tags reproduce the camelCase keys the .NET CosmosUserProfileRepository writes
// (its serializer context uses the CamelCase naming policy), so a Go-written
// document is byte-compatible with the existing container and an existing
// document hydrates here unchanged.
//
// Partition key: the Users container is partitioned by /id, which equals the
// Auth0 user id. Every read/write is therefore a single-partition point
// operation keyed on UserID.
type profileDocument struct {
	ID     string  `json:"id"`
	UserID string  `json:"userId"`
	Email  *string `json:"email"`

	PushEnabled bool         `json:"pushEnabled"`
	DigestDay   time.Weekday `json:"digestDay"`

	// emailDigestEnabled / savedDecision* are pointers so a legacy document that
	// predates these fields hydrates as opt-in (true) rather than the Go zero
	// value (false) — mirroring .NET's bool? coalesce-to-true on read.
	EmailDigestEnabled *bool `json:"emailDigestEnabled"`
	SavedDecisionPush  *bool `json:"savedDecisionPush"`
	SavedDecisionEmail *bool `json:"savedDecisionEmail"`

	ZonePreferences map[string]zonePreferencesDocument `json:"zonePreferences"`

	Tier                  string     `json:"tier"`
	SubscriptionExpiry    *time.Time `json:"subscriptionExpiry"`
	OriginalTransactionID *string    `json:"originalTransactionId"`
	GracePeriodExpiry     *time.Time `json:"gracePeriodExpiry"`
	LastActiveAt          time.Time  `json:"lastActiveAt"`
	// watchZoneCount is the CAS quota counter. omitempty so legacy documents
	// (written before this field existed) remain unchanged on re-read.
	WatchZoneCount *int `json:"watchZoneCount,omitempty"`
}

type zonePreferencesDocument struct {
	NewApplicationPush  bool `json:"newApplicationPush"`
	NewApplicationEmail bool `json:"newApplicationEmail"`
	DecisionPush        bool `json:"decisionPush"`
	DecisionEmail       bool `json:"decisionEmail"`
}

// newProfileDocument maps a domain profile to its persistence shape. The Cosmos
// document id equals the user id (the partition key), so a point read needs only
// the user id, matching .NET's FromDomain.
func newProfileDocument(p *UserProfile) profileDocument {
	zones := make(map[string]zonePreferencesDocument, len(p.ZonePreferences))
	for id, z := range p.ZonePreferences {
		zones[id] = zonePreferencesDocument(z)
	}
	emailDigest := p.Preferences.EmailDigestEnabled
	savedPush := p.Preferences.SavedDecisionPush
	savedEmail := p.Preferences.SavedDecisionEmail
	return profileDocument{
		ID:                    p.UserID,
		UserID:                p.UserID,
		Email:                 p.Email,
		PushEnabled:           p.Preferences.PushEnabled,
		DigestDay:             p.Preferences.DigestDay,
		EmailDigestEnabled:    &emailDigest,
		SavedDecisionPush:     &savedPush,
		SavedDecisionEmail:    &savedEmail,
		ZonePreferences:       zones,
		Tier:                  p.Tier.String(),
		SubscriptionExpiry:    p.SubscriptionExpiry,
		OriginalTransactionID: p.OriginalTransactionID,
		GracePeriodExpiry:     p.GracePeriodExpiry,
		LastActiveAt:          p.LastActiveAt,
		WatchZoneCount:        p.WatchZoneCount,
	}
}

// toDomain reconstitutes a domain profile from its stored document, coalescing
// the legacy-nullable preference flags to true exactly as .NET does on read.
func (d profileDocument) toDomain() (*UserProfile, error) {
	tier, err := ParseSubscriptionTier(d.Tier)
	if err != nil {
		return nil, err
	}
	zones := make(map[string]ZonePreferences, len(d.ZonePreferences))
	for id, z := range d.ZonePreferences {
		zones[id] = ZonePreferences(z)
	}
	return &UserProfile{
		UserID: d.UserID,
		Email:  d.Email,
		Preferences: NotificationPreferences{
			PushEnabled:        d.PushEnabled,
			DigestDay:          d.DigestDay,
			EmailDigestEnabled: coalesceTrue(d.EmailDigestEnabled),
			SavedDecisionPush:  coalesceTrue(d.SavedDecisionPush),
			SavedDecisionEmail: coalesceTrue(d.SavedDecisionEmail),
		},
		ZonePreferences:       zones,
		Tier:                  tier,
		SubscriptionExpiry:    d.SubscriptionExpiry,
		OriginalTransactionID: d.OriginalTransactionID,
		GracePeriodExpiry:     d.GracePeriodExpiry,
		LastActiveAt:          d.LastActiveAt,
		WatchZoneCount:        d.WatchZoneCount,
	}, nil
}

// coalesceTrue defaults an absent (nil) nullable flag to true, preserving the
// opt-in default for documents written before the flag existed.
func coalesceTrue(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}
