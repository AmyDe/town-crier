package offercodes

import (
	"errors"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// ErrNotFound signals that no offer code exists for the given canonical code.
var ErrNotFound = errors.New("offer code not found")

// ErrAlreadyRedeemed is returned by RedeemWithCAS when the code has reached its
// redemption cap (every slot consumed by some user).
var ErrAlreadyRedeemed = errors.New("offer code has already been redeemed")

// ErrAlreadyRedeemedByUser is returned by RedeemWithCAS when the calling user
// has already redeemed this code. It is distinct from ErrAlreadyRedeemed, but
// the redeem handler maps both to 409 code_already_redeemed.
var ErrAlreadyRedeemedByUser = errors.New("user has already redeemed this offer code")

const maxLabelLength = 100

const (
	minMaxRedemptions = 1
	maxMaxRedemptions = 10000
)

// OfferCode is a redeemable code granting a paid tier for a fixed duration, up
// to MaxRedemptions distinct users. NewOfferCode enforces the invariants for
// freshly minted codes.
//
// RedemptionCount survives GDPR anonymisation and is never decremented, so a
// consumed slot can never be reclaimed by a new redeemer.
type OfferCode struct {
	Code            string
	Tier            profiles.SubscriptionTier
	DurationDays    int
	CreatedAt       time.Time
	Label           string
	MaxRedemptions  int
	RedemptionCount int
}

// NewOfferCode mints a code, validating the canonical format, a non-Free tier,
// a 1..365 day duration, a required label (non-empty after trimming, at most
// 100 characters), and a redemption cap between 1 and 10,000 inclusive.
func NewOfferCode(code string, tier profiles.SubscriptionTier, durationDays int, label string, maxRedemptions int, createdAt time.Time) (OfferCode, error) {
	if !IsValidCanonical(code) {
		return OfferCode{}, errors.New("code is not a valid canonical offer code")
	}
	if tier == profiles.TierFree {
		return OfferCode{}, errors.New("offer codes cannot grant the free tier")
	}
	if durationDays < 1 || durationDays > 365 {
		return OfferCode{}, errors.New("duration must be between 1 and 365 days")
	}
	trimmedLabel := strings.TrimSpace(label)
	if trimmedLabel == "" {
		return OfferCode{}, errors.New("label is required")
	}
	if len(trimmedLabel) > maxLabelLength {
		return OfferCode{}, errors.New("label must be 100 characters or fewer")
	}
	if maxRedemptions < minMaxRedemptions || maxRedemptions > maxMaxRedemptions {
		return OfferCode{}, errors.New("max redemptions must be between 1 and 10000")
	}
	return OfferCode{
		Code:           code,
		Tier:           tier,
		DurationDays:   durationDays,
		CreatedAt:      createdAt,
		Label:          trimmedLabel,
		MaxRedemptions: maxRedemptions,
	}, nil
}

// IsFullyRedeemed reports whether every redemption slot has been consumed.
func (c *OfferCode) IsFullyRedeemed() bool { return c.RedemptionCount >= c.MaxRedemptions }

// Redemption is one user's claim on a code: which code, who redeemed it (nil
// after GDPR anonymisation), and when (also nil after anonymisation — the
// child row survives as a tombstone, but its PII is scrubbed).
type Redemption struct {
	Code       string
	UserID     *string
	RedeemedAt *time.Time
}

// ActiveAt reports whether this redemption's granted-tier window is still open
// at now: the redemption instant plus durationDays (from the redemption's code)
// has not yet passed. A scrubbed RedeemedAt is never active.
func (r Redemption) ActiveAt(now time.Time, durationDays int) bool {
	if r.RedeemedAt == nil {
		return false
	}
	return r.RedeemedAt.Add(time.Duration(durationDays) * 24 * time.Hour).After(now)
}

// RedeemedOfferCode is one redemption joined with its code's tier and
// duration, as returned by RedeemedByUserID and RedeemedByUsers. UserID is set
// only by RedeemedByUsers.
type RedeemedOfferCode struct {
	Code         string
	Tier         profiles.SubscriptionTier
	DurationDays int
	RedeemedAt   *time.Time
	UserID       *string
}

// ActiveAt reports whether this specific redemption's grant window is still
// open at now, per Redemption.ActiveAt.
func (r RedeemedOfferCode) ActiveAt(now time.Time) bool {
	return Redemption{Code: r.Code, RedeemedAt: r.RedeemedAt}.ActiveAt(now, r.DurationDays)
}

// ListedOfferCode is one row of the admin code listing: a code's own fields
// plus the most recent redemption time across every redeemer (nil if the code
// has never been redeemed).
type ListedOfferCode struct {
	OfferCode
	LastRedeemedAt *time.Time
}
