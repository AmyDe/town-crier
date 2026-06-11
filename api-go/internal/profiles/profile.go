// Package profiles owns the user-profile feature: the domain model, the Cosmos
// store, the /v1/me HTTP handlers, and the Auth0 Management (M2M) client used to
// keep Auth0's subscription_tier metadata in sync. It mirrors the .NET
// TownCrier.{Domain,Application,Infrastructure}.UserProfiles slices (GH#418
// iteration 3) but follows idiomatic Go: a plain struct validated at
// construction, a consumer-side store interface, and hand-written test fakes.
package profiles

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// SubscriptionTier enumerates the entitlement levels. The string forms ("Free",
// "Personal", "Pro") are the exact values the .NET SubscriptionTier enum
// serialises to on the wire and stores in Cosmos, so they are preserved here.
type SubscriptionTier int

const (
	// TierFree is the default, unpaid tier.
	TierFree SubscriptionTier = iota
	// TierPersonal is the £1.99/mo tier.
	TierPersonal
	// TierPro is the £5.99/mo tier (also the auto-grant target for pro domains).
	TierPro
)

// String returns the canonical wire/storage form of the tier.
func (t SubscriptionTier) String() string {
	switch t {
	case TierFree:
		return "Free"
	case TierPersonal:
		return "Personal"
	case TierPro:
		return "Pro"
	default:
		return "Free"
	}
}

// IsPaid reports whether the tier grants paid entitlements (anything but Free).
func (t SubscriptionTier) IsPaid() bool { return t != TierFree }

// ErrUnknownTier is returned by ParseSubscriptionTier for an unrecognised value.
var ErrUnknownTier = errors.New("unknown subscription tier")

// ParseSubscriptionTier converts a stored/wire tier string back to the enum.
// The match is exact and case-sensitive, mirroring .NET's Enum.Parse on the
// PascalCase stored value.
func ParseSubscriptionTier(s string) (SubscriptionTier, error) {
	switch s {
	case "Free":
		return TierFree, nil
	case "Personal":
		return TierPersonal, nil
	case "Pro":
		return TierPro, nil
	default:
		return TierFree, fmt.Errorf("%w: %q", ErrUnknownTier, s)
	}
}

// NotificationPreferences captures the user's global notification settings.
// Mirrors the .NET NotificationPreferences record; defaults are push-on,
// Monday digest, all email/saved-decision channels on.
type NotificationPreferences struct {
	PushEnabled        bool
	DigestDay          time.Weekday
	EmailDigestEnabled bool
	SavedDecisionPush  bool
	SavedDecisionEmail bool
}

// DefaultPreferences returns the .NET NotificationPreferences.Default value.
func DefaultPreferences() NotificationPreferences {
	return NotificationPreferences{
		PushEnabled:        true,
		DigestDay:          time.Monday,
		EmailDigestEnabled: true,
		SavedDecisionPush:  true,
		SavedDecisionEmail: true,
	}
}

// ZonePreferences captures per-watch-zone notification settings exported in the
// GDPR data dump. Watch zones themselves arrive in a later iteration; this type
// exists so the export contract can render the zone-preferences map.
type ZonePreferences struct {
	NewApplicationPush  bool
	NewApplicationEmail bool
	DecisionPush        bool
	DecisionEmail       bool
}

// UserProfile is the user-profile aggregate. Exported fields keep it a plain Go
// value; the constructor enforces the only real invariant (non-blank user id),
// and the small mutators preserve .NET's behaviours (forward-only activity,
// non-overwriting email backfill).
type UserProfile struct {
	UserID                string
	Email                 *string
	Preferences           NotificationPreferences
	ZonePreferences       map[string]ZonePreferences
	Tier                  SubscriptionTier
	SubscriptionExpiry    *time.Time
	OriginalTransactionID *string
	GracePeriodExpiry     *time.Time
	LastActiveAt          time.Time
}

// NewProfile registers a fresh profile with default preferences and the Free
// tier, mirroring .NET UserProfile.Register. A blank user id is rejected; a
// blank email is stored as nil (absent), not an empty string.
func NewProfile(userID, email string, now time.Time) (*UserProfile, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user id is required")
	}
	return &UserProfile{
		UserID:          userID,
		Email:           normaliseEmail(email),
		Preferences:     DefaultPreferences(),
		ZonePreferences: map[string]ZonePreferences{},
		Tier:            TierFree,
		LastActiveAt:    now,
	}, nil
}

// RecordActivity advances LastActiveAt to now only when now is later, matching
// .NET's forward-only RecordActivity. The dormancy-cleanup worker relies on this
// timestamp (UK GDPR Art. 5(1)(e)).
func (p *UserProfile) RecordActivity(now time.Time) {
	if now.After(p.LastActiveAt) {
		p.LastActiveAt = now
	}
}

// BackfillEmail sets the email only if it is currently absent. An already-set
// email is never overwritten, mirroring .NET BackfillEmail.
func (p *UserProfile) BackfillEmail(email string) {
	if p.Email != nil || strings.TrimSpace(email) == "" {
		return
	}
	p.Email = normaliseEmail(email)
}

// UpdatePreferences replaces the global notification preferences.
func (p *UserProfile) UpdatePreferences(prefs NotificationPreferences) {
	p.Preferences = prefs
}

// ActivateSubscription moves the profile to a paid tier with the given expiry
// and clears any grace period, mirroring .NET ActivateSubscription.
func (p *UserProfile) ActivateSubscription(tier SubscriptionTier, expiry time.Time) {
	p.Tier = tier
	exp := expiry
	p.SubscriptionExpiry = &exp
	p.GracePeriodExpiry = nil
}

func normaliseEmail(email string) *string {
	if strings.TrimSpace(email) == "" {
		return nil
	}
	e := email
	return &e
}
