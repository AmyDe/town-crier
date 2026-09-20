// Package profiles owns the user-profile feature: the domain model, the Postgres
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

// ErrNotFound signals that no profile exists for the given user id. Callers use
// errors.Is to translate it to a 404 response.
var ErrNotFound = errors.New("user profile not found")

// Page is one page of the admin user list: the profiles on this page plus the
// continuation token for the next page (empty when exhausted).
type Page struct {
	Profiles          []*UserProfile
	ContinuationToken string
}

// SubscriptionTier enumerates the entitlement levels. The string forms ("Free",
// "Personal", "Pro") are the canonical values stored in Cosmos and served on the
// wire.
type SubscriptionTier int

const (
	// TierFree is the default, unpaid tier.
	TierFree SubscriptionTier = iota
	// TierPersonal is the £1.99/mo tier.
	TierPersonal
	// TierPro is the £4.99/mo tier.
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

// AllowsCustomBoundary reports whether the tier may draw a custom-shape
// (polygon) watch zone boundary rather than a plain circle: Personal and Pro,
// never Free. Part of the custom-shape watch zones entitlement (epic GH#1031).
func (t SubscriptionTier) AllowsCustomBoundary() bool { return t.IsPaid() }

// AllowsWatchZoneFilter reports whether the tier may set a pre-canned
// keyword filter on a watch zone: Pro only, never Free or Personal. Narrower
// than AllowsCustomBoundary (which any paid tier grants via IsPaid) —
// pre-canned filters are a deliberate Pro-tier-exclusive upsell (GH#1090,
// epic tc-w825j), not a Personal-tier capability.
func (t SubscriptionTier) AllowsWatchZoneFilter() bool { return t.IsPaidPro() }

// MaxZoneRadiusMetres returns the maximum enclosing-circle radius, in metres,
// permitted for a custom-shape polygon boundary: Free=2000, Personal=5000,
// Pro=10000, mirroring mobile/ios/packages/town-crier-domain/Sources/
// ValueObjects/WatchZoneLimits.swift. This bounds only the polygon's
// enclosing radius — it is unrelated to the existing flat circle-zone radius
// ceiling in internal/watchzones/nearby.go, which this method does not touch.
func (t SubscriptionTier) MaxZoneRadiusMetres() float64 {
	switch t {
	case TierPersonal:
		return 5000
	case TierPro:
		return 10000
	default:
		return 2000
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
	// LifetimeTier is the tier granted by a one-off lifetime purchase, independent
	// of any subscription. Invariant: Tier is never lower than LifetimeTier, so
	// readers of the raw stored Tier stay correct without knowing about lifetime.
	LifetimeTier                  SubscriptionTier
	LifetimeOriginalTransactionID *string
	LifetimePurchasedAt           *time.Time
	// SubscriptionProductID is the App Store product behind the current
	// subscription window. It is nil for offer-code and admin grants.
	SubscriptionProductID *string
	LastActiveAt          time.Time
	// CreatedAt is the profile's creation time. It is owned by the database
	// (users.created_at DEFAULT CURRENT_TIMESTAMP) and read-only in Go: Save
	// never writes it. NewProfile stamps it for the in-memory create response,
	// but the DB DEFAULT is authoritative on persistence.
	CreatedAt time.Time
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
		CreatedAt:       now,
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
// within its window (including a live grace period and the far-future admin
// grant) are returned unchanged. The result is never lower than LifetimeTier,
// which no expiry can revoke.
//
// Every entitlement gate reads this, never the raw stored Tier, so an offer-code
// grant that has run out — or an App Store sub past expiry whose webhook never
// arrived — is treated as Free everywhere without mutating the stored document
// (the daily sweep, Phase 2, reverts the stored state separately).
func (p *UserProfile) EffectiveTier(now time.Time) SubscriptionTier {
	return max(p.subscriptionTier(now), p.LifetimeTier)
}

func (p *UserProfile) subscriptionTier(now time.Time) SubscriptionTier {
	if p.Tier == TierFree {
		return TierFree
	}
	if p.SubscriptionExpiry == nil {
		// A lifetime-only profile is paid with no expiry. Any other paid tier
		// without an expiry is malformed; treat it as still-entitled rather than
		// silently downgrade (no proof of expiry).
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
// and clears any grace period and App Store product. The stored Tier never drops
// below LifetimeTier. Used by offer-code redemption and admin grants.
func (p *UserProfile) ActivateSubscription(tier SubscriptionTier, expiry time.Time) {
	p.Tier = max(tier, p.LifetimeTier)
	exp := expiry
	p.SubscriptionExpiry = &exp
	p.GracePeriodExpiry = nil
	p.SubscriptionProductID = nil
}

// ActivateAppStoreSubscription is ActivateSubscription for an App Store
// subscription: it also records the product ID behind the subscription window.
func (p *UserProfile) ActivateAppStoreSubscription(tier SubscriptionTier, expiry time.Time, productID string) {
	p.ActivateSubscription(tier, expiry)
	id := productID
	p.SubscriptionProductID = &id
}

// ExpireSubscription ends the subscription entitlement: it clears the expiry,
// grace period and App Store product, and drops the stored Tier to LifetimeTier
// (Free when the profile holds no lifetime purchase). OriginalTransactionID is
// left in place so a later notification can still find the profile.
func (p *UserProfile) ExpireSubscription() {
	p.Tier = p.LifetimeTier
	p.SubscriptionExpiry = nil
	p.GracePeriodExpiry = nil
	p.SubscriptionProductID = nil
}

// GrantLifetime records a lifetime purchase and raises the stored Tier to at
// least tier. Granting the same purchase again is a no-op in effect.
func (p *UserProfile) GrantLifetime(tier SubscriptionTier, originalTransactionID string, purchasedAt time.Time) {
	p.LifetimeTier = tier
	id := originalTransactionID
	p.LifetimeOriginalTransactionID = &id
	at := purchasedAt
	p.LifetimePurchasedAt = &at
	p.Tier = max(p.Tier, tier)
}

// RevokeLifetime removes a lifetime purchase (an Apple refund or revocation).
// With no subscription window open the stored Tier drops to Free. Otherwise the
// stored Tier stays as it is: lazy expiry and the next App Store event settle it.
func (p *UserProfile) RevokeLifetime() {
	p.LifetimeTier = TierFree
	p.LifetimeOriginalTransactionID = nil
	p.LifetimePurchasedAt = nil
	if p.SubscriptionExpiry == nil {
		p.Tier = TierFree
	}
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
