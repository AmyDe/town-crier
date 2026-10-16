package subscriptionevents

import (
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

var totalsNow = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func future() *time.Time { t := totalsNow.Add(24 * time.Hour); return &t }
func past() *time.Time   { t := totalsNow.Add(-24 * time.Hour); return &t }

func TestComputeTotals_EmptyHasAllKeys(t *testing.T) {
	t.Parallel()
	got := ComputeTotals(0, nil, totalsNow)
	if got != (Totals{}) {
		t.Errorf("empty totals = %+v, want zero value", got)
	}
}

func TestComputeTotals_Buckets(t *testing.T) {
	t.Parallel()
	cands := []*profiles.UserProfile{
		{Tier: profiles.TierPersonal, SubscriptionExpiry: future(), OriginalTransactionID: strp("a"), SubscriptionProductID: strp("uk.towncrierapp.personal.monthly")},
		{Tier: profiles.TierPro, SubscriptionExpiry: future(), OriginalTransactionID: strp("b"), SubscriptionProductID: strp(productProAnnual)},
		{Tier: profiles.TierPro, SubscriptionExpiry: future(), OriginalTransactionID: strp("c"), SubscriptionProductID: strp(productProAnnual), SubscriptionInTrial: true},
		{Tier: profiles.TierPro, SubscriptionExpiry: future()},
		{Tier: profiles.TierPro, LifetimeTier: profiles.TierPro, LifetimeOriginalTransactionID: strp("l")},
		{Tier: profiles.TierPro, SubscriptionExpiry: past(), OriginalTransactionID: strp("d"), SubscriptionProductID: strp(productProAnnual)},
		{Tier: profiles.TierPersonal, SubscriptionExpiry: past(), GracePeriodExpiry: future(), OriginalTransactionID: strp("e"), SubscriptionProductID: strp("uk.towncrierapp.personal.monthly")},
	}
	got := ComputeTotals(100, cands, totalsNow)

	want := Totals{
		Users:   100,
		ByTier:  ByTier{Free: 100 - 6, Personal: 2, Pro: 4},
		ByPlan:  ByPlan{Personal: PlanCounts{Monthly: 2}, Pro: PlanCounts{Annual: 2, Lifetime: 1, Comped: 1}},
		Paying:  4,
		InTrial: 1,
		InGrace: 1,
		Comped:  1,
	}
	if got != want {
		t.Errorf("totals = %+v, want %+v", got, want)
	}
	if s := got.ByPlan.Personal.sum(); s != got.ByTier.Personal {
		t.Errorf("Personal plan sum %d != byTier %d", s, got.ByTier.Personal)
	}
	if s := got.ByPlan.Pro.sum(); s != got.ByTier.Pro {
		t.Errorf("Pro plan sum %d != byTier %d", s, got.ByTier.Pro)
	}
}
