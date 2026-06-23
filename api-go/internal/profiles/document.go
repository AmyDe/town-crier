package profiles

import (
	"time"
)

// profileDocument is the Cosmos persistence shape for a UserProfile. The JSON
// tags use camelCase to match the stored document shape, so documents in the
// existing container hydrate here unchanged.
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
	// value (false) — see coalesceTrue.
	EmailDigestEnabled *bool `json:"emailDigestEnabled"`
	SavedDecisionPush  *bool `json:"savedDecisionPush"`
	SavedDecisionEmail *bool `json:"savedDecisionEmail"`

	ZonePreferences map[string]zonePreferencesDocument `json:"zonePreferences"`

	Tier                  string     `json:"tier"`
	SubscriptionExpiry    *time.Time `json:"subscriptionExpiry"`
	OriginalTransactionID *string    `json:"originalTransactionId"`
	GracePeriodExpiry     *time.Time `json:"gracePeriodExpiry"`
	LastActiveAt          time.Time  `json:"lastActiveAt"`
	// LastActiveAtEpoch is LastActiveAt as Unix epoch milliseconds — a numeric
	// mirror written on every upsert so the dormant scan filters server-side on a
	// value that sorts unambiguously. lastActiveAt itself is persisted in two wire
	// formats ("+00:00" and "Z") that do not sort lexicographically, so a SQL
	// string comparison on it would silently miss "Z"-stored accounts. It is a
	// derived query-acceleration field: LastActiveAt remains the source of truth,
	// so toDomain reads the timestamp, not this mirror. No omitempty — the key must
	// always be present so a server-side IS_DEFINED check can tell a freshly
	// written doc from a legacy, un-backfilled one.
	LastActiveAtEpoch int64 `json:"lastActiveAtEpoch"`
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
// the user id.
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
		LastActiveAtEpoch:     p.LastActiveAt.UnixMilli(),
		WatchZoneCount:        p.WatchZoneCount,
	}
}

// toDomain reconstitutes a domain profile from its stored document, coalescing
// the legacy-nullable preference flags to true (see coalesceTrue).
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
