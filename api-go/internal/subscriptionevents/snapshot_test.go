package subscriptionevents

import (
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

func strp(s string) *string { return &s }

func TestSnapshotOf_PlanRule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		p    profiles.UserProfile
		want Snapshot
	}{
		{"free", profiles.UserProfile{Tier: profiles.TierFree}, Snapshot{Tier: profiles.TierFree, Plan: PlanNone}},
		{"lifetime wins over active subscription", profiles.UserProfile{
			Tier: profiles.TierPro, LifetimeTier: profiles.TierPro, LifetimeOriginalTransactionID: strp("l"),
			SubscriptionProductID: strp(productProAnnual),
		}, Snapshot{Tier: profiles.TierPro, Plan: PlanLifetime}},
		{"annual", profiles.UserProfile{Tier: profiles.TierPro, SubscriptionProductID: strp(productProAnnual)},
			Snapshot{Tier: profiles.TierPro, Plan: PlanAnnual}},
		{"monthly", profiles.UserProfile{Tier: profiles.TierPersonal, SubscriptionProductID: strp("uk.towncrierapp.personal.monthly")},
			Snapshot{Tier: profiles.TierPersonal, Plan: PlanMonthly}},
		{"offer code grant after app store sub (product cleared)", profiles.UserProfile{Tier: profiles.TierPro, OriginalTransactionID: strp("o")},
			Snapshot{Tier: profiles.TierPro, Plan: PlanComped}},
		{"trial", profiles.UserProfile{Tier: profiles.TierPro, SubscriptionProductID: strp(productProAnnual), SubscriptionInTrial: true},
			Snapshot{Tier: profiles.TierPro, Plan: PlanAnnual, InTrial: true}},
		{"lifetime ignores trial flag", profiles.UserProfile{
			Tier: profiles.TierPro, LifetimeTier: profiles.TierPro, LifetimeOriginalTransactionID: strp("l"), SubscriptionInTrial: true,
		}, Snapshot{Tier: profiles.TierPro, Plan: PlanLifetime}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := SnapshotOf(&tc.p); got != tc.want {
				t.Errorf("SnapshotOf = %+v, want %+v", got, tc.want)
			}
		})
	}
}
