package profiles

import (
	"reflect"
	"testing"
	"time"
)

func TestUserProfile_RenewSubscription(t *testing.T) {
	t.Parallel()
	p := &UserProfile{Tier: TierPro}
	grace := time.Now()
	p.GracePeriodExpiry = &grace

	newExpiry := time.Now().AddDate(0, 1, 0).UTC()
	p.RenewSubscription(newExpiry)

	if p.SubscriptionExpiry == nil || !p.SubscriptionExpiry.Equal(newExpiry) {
		t.Errorf("SubscriptionExpiry = %v, want %v", p.SubscriptionExpiry, newExpiry)
	}
	if p.GracePeriodExpiry != nil {
		t.Errorf("GracePeriodExpiry = %v, want nil", p.GracePeriodExpiry)
	}
	// Renewal does not change the tier.
	if p.Tier != TierPro {
		t.Errorf("Tier = %v, want Pro", p.Tier)
	}
}

func TestUserProfile_EnterGracePeriod(t *testing.T) {
	t.Parallel()
	p := &UserProfile{Tier: TierPersonal}
	graceEnd := time.Now().AddDate(0, 0, 16).UTC()

	p.EnterGracePeriod(graceEnd)

	if p.GracePeriodExpiry == nil || !p.GracePeriodExpiry.Equal(graceEnd) {
		t.Errorf("GracePeriodExpiry = %v, want %v", p.GracePeriodExpiry, graceEnd)
	}
	// Entering grace keeps the tier and expiry — the entitlement persists.
	if p.Tier != TierPersonal {
		t.Errorf("Tier = %v, want Personal", p.Tier)
	}
}

func TestUserProfile_LinkOriginalTransactionID(t *testing.T) {
	t.Parallel()
	p := &UserProfile{}
	p.LinkOriginalTransactionID("orig-123")

	if p.OriginalTransactionID == nil || *p.OriginalTransactionID != "orig-123" {
		t.Errorf("OriginalTransactionID = %v, want orig-123", p.OriginalTransactionID)
	}
}

func TestSubscriptionTier_Entitlements(t *testing.T) {
	t.Parallel()
	paid := []string{"StatusChangeAlerts", "DecisionUpdateAlerts", "HourlyDigestEmails"}
	tests := []struct {
		tier SubscriptionTier
		want []string
	}{
		{TierFree, []string{}},
		{TierPersonal, paid},
		{TierPro, paid},
	}
	for _, tc := range tests {
		t.Run(tc.tier.String(), func(t *testing.T) {
			t.Parallel()
			if got := tc.tier.Entitlements(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Entitlements() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSubscriptionTier_WatchZoneLimit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		tier SubscriptionTier
		want int
	}{
		{TierFree, 1},
		{TierPersonal, 3},
		{TierPro, 2147483647},
	}
	for _, tc := range tests {
		t.Run(tc.tier.String(), func(t *testing.T) {
			t.Parallel()
			if got := tc.tier.WatchZoneLimit(); got != tc.want {
				t.Errorf("WatchZoneLimit() = %d, want %d", got, tc.want)
			}
		})
	}
}
