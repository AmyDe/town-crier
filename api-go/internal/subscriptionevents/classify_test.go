package subscriptionevents

import (
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

func TestClassify(t *testing.T) {
	t.Parallel()
	free := Snapshot{Tier: profiles.TierFree, Plan: PlanNone}
	proMonthly := Snapshot{Tier: profiles.TierPro, Plan: PlanMonthly}
	proTrial := Snapshot{Tier: profiles.TierPro, Plan: PlanAnnual, InTrial: true}
	proAnnual := Snapshot{Tier: profiles.TierPro, Plan: PlanAnnual}
	personal := Snapshot{Tier: profiles.TierPersonal, Plan: PlanMonthly}
	tests := []struct {
		name          string
		before, after Snapshot
		want          EventType
		wantOK        bool
	}{
		{"unchanged", proMonthly, proMonthly, "", false},
		{"free to trial", free, proTrial, EventTrialStarted, true},
		{"free to paid", free, proMonthly, EventPaidStarted, true},
		{"trial converts", proTrial, proAnnual, EventTrialConverted, true},
		{"upgrade", personal, proMonthly, EventUpgraded, true},
		{"downgrade", proMonthly, personal, EventDowngraded, true},
		{"plan change", proMonthly, proAnnual, EventPlanChanged, true},
		{"to lifetime", proMonthly, Snapshot{Tier: profiles.TierPro, Plan: PlanLifetime}, EventPlanChanged, true},
		{"revert to free", proMonthly, free, EventRevertedToFree, true},
		{"trial to free", proTrial, free, EventRevertedToFree, true},
		{"renewal or grace entry leaves snapshot unchanged", proAnnual, proAnnual, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := Classify(tc.before, tc.after)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("Classify = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
