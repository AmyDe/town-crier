package subscriptionevents

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// Totals is the whole-base subscription tally sent with every event. Every key
// is always present so a receiver never has to handle a missing field.
type Totals struct {
	Users   int    `json:"users"`
	ByTier  ByTier `json:"byTier"`
	ByPlan  ByPlan `json:"byPlan"`
	Paying  int    `json:"paying"`
	InTrial int    `json:"inTrial"`
	InGrace int    `json:"inGrace"`
	Comped  int    `json:"comped"`
}

// ByTier counts profiles by effective tier.
type ByTier struct {
	Free     int `json:"Free"`
	Personal int `json:"Personal"`
	Pro      int `json:"Pro"`
}

// ByPlan counts effective-paid profiles per paid tier per plan.
type ByPlan struct {
	Personal PlanCounts `json:"Personal"`
	Pro      PlanCounts `json:"Pro"`
}

// PlanCounts counts profiles per plan within one tier.
type PlanCounts struct {
	Monthly  int `json:"monthly"`
	Annual   int `json:"annual"`
	Lifetime int `json:"lifetime"`
	Comped   int `json:"comped"`
}

func (c PlanCounts) sum() int { return c.Monthly + c.Annual + c.Lifetime + c.Comped }

// ComputeTotals tallies the base. total is every profile; candidates are the
// stored-paid profiles. A candidate counts as paid only while its effective
// tier at now is paid, so lapsed profiles fall into Free.
func ComputeTotals(total int, candidates []*profiles.UserProfile, now time.Time) Totals {
	t := Totals{Users: total}
	paid := 0
	for _, c := range candidates {
		effective := c.EffectiveTier(now)
		if !effective.IsPaid() {
			continue
		}
		paid++

		plan := planOf(c)
		var counts *PlanCounts
		switch effective {
		case profiles.TierPersonal:
			t.ByTier.Personal++
			counts = &t.ByPlan.Personal
		case profiles.TierPro:
			t.ByTier.Pro++
			counts = &t.ByPlan.Pro
		}
		trial := c.OriginalTransactionID != nil && c.SubscriptionInTrial && (plan == PlanMonthly || plan == PlanAnnual)

		switch plan {
		case PlanMonthly:
			counts.Monthly++
		case PlanAnnual:
			counts.Annual++
		case PlanLifetime:
			counts.Lifetime++
		default:
			counts.Comped++
			t.Comped++
		}
		if trial {
			t.InTrial++
		} else if plan != PlanComped {
			t.Paying++
		}
		if inGrace(c, now) {
			t.InGrace++
		}
	}
	t.ByTier.Free = total - paid
	return t
}

func inGrace(c *profiles.UserProfile, now time.Time) bool {
	return c.SubscriptionExpiry != nil && !c.SubscriptionExpiry.After(now) &&
		c.GracePeriodExpiry != nil && c.GracePeriodExpiry.After(now)
}
