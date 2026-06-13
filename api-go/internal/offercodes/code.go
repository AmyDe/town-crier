package offercodes

import (
	"errors"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// ErrAlreadyRedeemed is returned by Redeem when the code has already been
// claimed. The redeem handler detects the already-redeemed state before calling
// Redeem and builds the wire 409 message itself; this guards the invariant.
var ErrAlreadyRedeemed = errors.New("offer code has already been redeemed")

// OfferCode is a single redeemable code granting a paid tier for a fixed
// duration, mirroring the .NET OfferCode aggregate. Fields are exported so the
// store can rehydrate a stored document directly; NewOfferCode enforces the
// invariants for freshly minted codes.
type OfferCode struct {
	Code             string
	Tier             profiles.SubscriptionTier
	DurationDays     int
	CreatedAt        time.Time
	RedeemedByUserID *string
	RedeemedAt       *time.Time
}

// NewOfferCode mints a code, validating the canonical format, a non-Free tier,
// and a 1..365 day duration — the same invariants the .NET OfferCode
// constructor enforces.
func NewOfferCode(code string, tier profiles.SubscriptionTier, durationDays int, createdAt time.Time) (OfferCode, error) {
	if !IsValidCanonical(code) {
		return OfferCode{}, errors.New("code is not a valid canonical offer code")
	}
	if tier == profiles.TierFree {
		return OfferCode{}, errors.New("offer codes cannot grant the free tier")
	}
	if durationDays < 1 || durationDays > 365 {
		return OfferCode{}, errors.New("duration must be between 1 and 365 days")
	}
	return OfferCode{Code: code, Tier: tier, DurationDays: durationDays, CreatedAt: createdAt}, nil
}

// IsRedeemed reports whether the code has been claimed.
func (c *OfferCode) IsRedeemed() bool { return c.RedeemedByUserID != nil }

// Redeem claims the code for userID at now. A second redemption is rejected with
// ErrAlreadyRedeemed.
func (c *OfferCode) Redeem(userID string, now time.Time) error {
	if c.IsRedeemed() {
		return ErrAlreadyRedeemed
	}
	uid := userID
	at := now
	c.RedeemedByUserID = &uid
	c.RedeemedAt = &at
	return nil
}
