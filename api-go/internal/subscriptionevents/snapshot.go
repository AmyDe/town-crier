package subscriptionevents

import "github.com/AmyDe/town-crier/api-go/internal/profiles"

// productProAnnual mirrors subscriptions.ProductProAnnual; this package cannot
// import subscriptions because subscriptions depends on it. A test in
// subscriptions pins the two together.
const productProAnnual = "uk.towncrierapp.pro.annual"

// Plan is the billing arrangement behind a paid tier.
type Plan string

// Plan values as they appear on the wire.
const (
	PlanNone     Plan = "none"
	PlanMonthly  Plan = "monthly"
	PlanAnnual   Plan = "annual"
	PlanLifetime Plan = "lifetime"
	PlanComped   Plan = "comped"
)

// Snapshot is the stored subscription state of a profile at one moment.
type Snapshot struct {
	Tier    profiles.SubscriptionTier
	Plan    Plan
	InTrial bool
}

// SnapshotOf reads the profile's stored (not effective) subscription state. The
// sweep only sees profiles whose effective tier has already lapsed, so an
// effective-tier snapshot would hide every sweep downgrade.
func SnapshotOf(p *profiles.UserProfile) Snapshot {
	plan := planOf(p)
	return Snapshot{
		Tier:    p.Tier,
		Plan:    plan,
		InTrial: (plan == PlanMonthly || plan == PlanAnnual) && p.SubscriptionInTrial,
	}
}

func planOf(p *profiles.UserProfile) Plan {
	switch {
	case p.LifetimeOriginalTransactionID != nil && p.LifetimeTier.IsPaid():
		return PlanLifetime
	case p.Tier == profiles.TierFree:
		return PlanNone
	case p.SubscriptionProductID != nil && *p.SubscriptionProductID == productProAnnual:
		return PlanAnnual
	case p.SubscriptionProductID != nil:
		return PlanMonthly
	default:
		return PlanComped
	}
}
