// Package profiles owns the user-profile feature: the domain model, the Cosmos
// store, the /v1/me HTTP handlers, and the Auth0 Management (M2M) client used to
// keep Auth0's subscription_tier metadata in sync. It follows idiomatic Go: a
// plain struct validated at construction, a consumer-side store interface, and
// hand-written test fakes.
package profiles

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// SubscriptionTier enumerates the entitlement levels. The string forms ("Free",
// "Personal", "Pro") are the canonical values stored in Cosmos and served on the
// wire.
type SubscriptionTier int

const (
	// TierFree is the default, unpaid tier.
	TierFree SubscriptionTier = iota
	// TierPersonal is the £1.99/mo tier.
	TierPersonal
	// TierPro is the £5.99/mo tier.
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

// IsPaidPro reports whether the tier is specifically Pro. The weekly digest PUSH
// is Pro-only, distinct from the hourly-digest entitlement which Personal also
// holds.
func (t SubscriptionTier) IsPaidPro() bool { return t == TierPro }

// HasHourlyDigestEntitlement reports whether the tier grants the
// HourlyDigestEmails entitlement (Personal and Pro, never Free) — hourly digest
// emails are a paid, server-enforced entitlement.
func (t SubscriptionTier) HasHourlyDigestEntitlement() bool { return t.IsPaid() }

// unlimitedWatchZones is the Pro-tier watch-zone limit: 2147483647 (int32 max),
// the sentinel the iOS app reads as "no limit".
const unlimitedWatchZones = 2147483647

// Entitlements returns the entitlement strings granted by the tier: paid tiers
// grant the same three, Free grants none. The order is fixed so the
// /v1/subscriptions/verify response is stable.
func (t SubscriptionTier) Entitlements() []string {
	if t.IsPaid() {
		return []string{"StatusChangeAlerts", "DecisionUpdateAlerts", "HourlyDigestEmails"}
	}
	return []string{}
}

// WatchZoneLimit returns the maximum number of watch zones the tier permits:
// Free=1, Personal=3, Pro=unlimited.
func (t SubscriptionTier) WatchZoneLimit() int {
	switch t {
	case TierPersonal:
		return 3
	case TierPro:
		return unlimitedWatchZones
	default:
		return 1
	}
}

// ErrUnknownTier is returned by ParseSubscriptionTier for an unrecognised value.
var ErrUnknownTier = errors.New("unknown subscription tier")

// ParseSubscriptionTier converts a stored/wire tier string back to the enum.
// The match is exact and case-sensitive (PascalCase: "Free", "Personal", "Pro").
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
// Defaults are push-on, Monday digest, all email/saved-decision channels on.
type NotificationPreferences struct {
	PushEnabled        bool
	DigestDay          time.Weekday
	EmailDigestEnabled bool
	SavedDecisionPush  bool
	SavedDecisionEmail bool
}

// DefaultPreferences returns the default notification preferences.
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
// and the small mutators maintain forward-only activity and non-overwriting
// email backfill.
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
	// WatchZoneCount is the CAS-maintained quota counter. A nil value indicates
	// a legacy profile written before this field existed; the create path
	// initialises it on first use by reading the live zone count (lazy-init).
	WatchZoneCount *int
}

// NewProfile registers a fresh profile with default preferences and the Free
// tier. A blank user id is rejected; a blank email is stored as nil (absent),
// not an empty string.
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

// RecordActivity advances LastActiveAt to now only when now is later
// (forward-only). The dormancy-cleanup worker relies on this timestamp
// (UK GDPR Art. 5(1)(e)).
func (p *UserProfile) RecordActivity(now time.Time) {
	if now.After(p.LastActiveAt) {
		p.LastActiveAt = now
	}
}

// BackfillEmail sets the email only if it is currently absent. An already-set
// email is never overwritten.
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

// EffectiveTier returns the tier the user is actually entitled to at now,
// applying ADR 0010's lazy expiry rule: a paid tier whose SubscriptionExpiry has
// passed — with no grace period, or a grace period that has also passed —
// collapses to Free regardless of the stored Tier. Free and any paid tier still
// within its window (including a live grace period and the far-future pro-domain
// auto-grant) are returned unchanged.
//
// Every entitlement gate reads this, never the raw stored Tier, so an offer-code
// grant that has run out — or an App Store sub past expiry whose webhook never
// arrived — is treated as Free everywhere without mutating the stored document
// (the daily sweep, Phase 2, reverts the stored state separately).
func (p *UserProfile) EffectiveTier(now time.Time) SubscriptionTier {
	if p.Tier == TierFree {
		return TierFree
	}
	if p.SubscriptionExpiry == nil {
		// Invariant: every paid grant sets an expiry (ActivateSubscription /
		// pro-domain auto-grant). A paid tier with no expiry is malformed; treat
		// as still-entitled rather than silently downgrade (no proof of expiry).
		return p.Tier
	}
	// "expired" mirrors the lapsed-txn filter on the verify path: expired when
	// SubscriptionExpiry is NOT strictly after now (so expiry == now is expired).
	if !p.SubscriptionExpiry.After(now) {
		if p.GracePeriodExpiry == nil || !p.GracePeriodExpiry.After(now) {
			return TierFree
		}
	}
	return p.Tier
}

// ActivateSubscription moves the profile to a paid tier with the given expiry
// and clears any grace period.
func (p *UserProfile) ActivateSubscription(tier SubscriptionTier, expiry time.Time) {
	p.Tier = tier
	exp := expiry
	p.SubscriptionExpiry = &exp
	p.GracePeriodExpiry = nil
}

// ExpireSubscription drops the profile back to the Free tier and clears the
// subscription expiry and grace period. Used by the admin grant endpoint when
// granting the Free tier (a downgrade).
func (p *UserProfile) ExpireSubscription() {
	p.Tier = TierFree
	p.SubscriptionExpiry = nil
	p.GracePeriodExpiry = nil
}

// RenewSubscription extends the subscription to a new expiry and clears any
// grace period, without changing the tier. Applied on the App Store DID_RENEW
// notification.
func (p *UserProfile) RenewSubscription(newExpiry time.Time) {
	exp := newExpiry
	p.SubscriptionExpiry = &exp
	p.GracePeriodExpiry = nil
}

// EnterGracePeriod records the grace-period end while leaving the tier and
// expiry intact, so the entitlement persists through a billing retry. Applied
// on DID_FAIL_TO_RENEW with the GRACE_PERIOD subtype.
func (p *UserProfile) EnterGracePeriod(graceEnd time.Time) {
	end := graceEnd
	p.GracePeriodExpiry = &end
}

// LinkOriginalTransactionID records the Apple original transaction ID so App
// Store Server Notifications can later locate this profile cross-partition.
// The caller supplies a non-blank ID (the transaction decoder requires
// originalTransactionId).
func (p *UserProfile) LinkOriginalTransactionID(originalTransactionID string) {
	id := originalTransactionID
	p.OriginalTransactionID = &id
}

func normaliseEmail(email string) *string {
	if strings.TrimSpace(email) == "" {
		return nil
	}
	e := email
	return &e
}
