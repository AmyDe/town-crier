package offercodes

import (
	"errors"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// ErrNotFound signals that no offer code exists for the given canonical code.
var ErrNotFound = errors.New("offer code not found")

// ErrAlreadyRedeemed is returned by Redeem when the code has already been
// claimed. The redeem handler detects the already-redeemed state before calling
// Redeem and builds the wire 409 message itself; this guards the invariant.
var ErrAlreadyRedeemed = errors.New("offer code has already been redeemed")

// OfferCode is a single redeemable code granting a paid tier for a fixed
// duration. Fields are exported so the store can rehydrate a stored document
// directly; NewOfferCode enforces the invariants for freshly minted codes.
//
// Redeemed is a consumed tombstone that survives GDPR Art. 17 anonymisation:
// when a redeemer is scrubbed from a redeemed code, RedeemedByUserID and
// RedeemedAt are nulled but Redeemed stays true so the code can never be
// re-redeemed (the campaign audit that it WAS used must survive). A freshly
// minted code is not redeemed; Redeem sets the tombstone alongside the redeemer.
type OfferCode struct {
	Code             string
	Tier             profiles.SubscriptionTier
	DurationDays     int
	CreatedAt        time.Time
	Redeemed         bool
	RedeemedByUserID *string
	RedeemedAt       *time.Time
}

// NewOfferCode mints a code, validating the canonical format, a non-Free tier,
// and a 1..365 day duration.
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

// IsRedeemed reports whether the code has been claimed. The consumed tombstone
// (Redeemed) is authoritative: an anonymised code keeps it set even though its
// RedeemedByUserID has been scrubbed, so it can never be re-redeemed.
func (c *OfferCode) IsRedeemed() bool { return c.Redeemed || c.RedeemedByUserID != nil }

// ActiveAt reports whether the code's granted tier window is still open at now:
// the redemption instant plus DurationDays has not yet passed. A code that was
// never redeemed (RedeemedAt == nil) is never active. This is the single
// authoritative "still-active offer window" rule reused by both the list-users
// offer-code column and the admin stats comped/active-offer aggregate, so the
// two surfaces can never disagree on whether a code still counts.
func (c *OfferCode) ActiveAt(now time.Time) bool {
	if c.RedeemedAt == nil {
		return false
	}
	return c.RedeemedAt.Add(time.Duration(c.DurationDays) * 24 * time.Hour).After(now)
}

// Redeem claims the code for userID at now. A second redemption is rejected with
// ErrAlreadyRedeemed.
func (c *OfferCode) Redeem(userID string, now time.Time) error {
	if c.IsRedeemed() {
		return ErrAlreadyRedeemed
	}
	uid := userID
	at := now
	c.Redeemed = true
	c.RedeemedByUserID = &uid
	c.RedeemedAt = &at
	return nil
}
