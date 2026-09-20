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

func TestSubscriptionTier_IsPaidPro(t *testing.T) {
	t.Parallel()
	tests := []struct {
		tier SubscriptionTier
		want bool
	}{
		{TierFree, false},
		{TierPersonal, false},
		{TierPro, true},
	}
	for _, tc := range tests {
		t.Run(tc.tier.String(), func(t *testing.T) {
			t.Parallel()
			if got := tc.tier.IsPaidPro(); got != tc.want {
				t.Errorf("IsPaidPro() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSubscriptionTier_HasHourlyDigestEntitlement(t *testing.T) {
	t.Parallel()
	// Hourly digest emails are a paid entitlement granted to Personal and Pro,
	// never Free (HourlyDigestEmails is in the paid-entitlements set).
	tests := []struct {
		tier SubscriptionTier
		want bool
	}{
		{TierFree, false},
		{TierPersonal, true},
		{TierPro, true},
	}
	for _, tc := range tests {
		t.Run(tc.tier.String(), func(t *testing.T) {
			t.Parallel()
			if got := tc.tier.HasHourlyDigestEntitlement(); got != tc.want {
				t.Errorf("HasHourlyDigestEntitlement() = %v, want %v", got, tc.want)
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

var lifetimeNow = time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)

func lifetimePro(t *testing.T) *UserProfile {
	t.Helper()
	p := &UserProfile{Tier: TierFree}
	p.GrantLifetime(TierPro, "life-1", lifetimeNow.Add(-24*time.Hour))
	return p
}

func TestProfile_GrantLifetime_RaisesTier(t *testing.T) {
	t.Parallel()
	purchased := lifetimeNow.Add(-time.Hour)
	tests := []struct {
		name string
		tier SubscriptionTier
	}{
		{"free", TierFree},
		{"personal", TierPersonal},
		{"pro", TierPro},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &UserProfile{Tier: tc.tier}

			p.GrantLifetime(TierPro, "life-1", purchased)

			if p.Tier != TierPro {
				t.Errorf("Tier = %v, want Pro", p.Tier)
			}
			if p.LifetimeTier != TierPro {
				t.Errorf("LifetimeTier = %v, want Pro", p.LifetimeTier)
			}
			if p.LifetimeOriginalTransactionID == nil || *p.LifetimeOriginalTransactionID != "life-1" {
				t.Errorf("LifetimeOriginalTransactionID = %v, want life-1", p.LifetimeOriginalTransactionID)
			}
			if p.LifetimePurchasedAt == nil || !p.LifetimePurchasedAt.Equal(purchased) {
				t.Errorf("LifetimePurchasedAt = %v, want %v", p.LifetimePurchasedAt, purchased)
			}
		})
	}
}

func TestProfile_GrantLifetime_Idempotent(t *testing.T) {
	t.Parallel()
	p := &UserProfile{Tier: TierFree}
	purchased := lifetimeNow.Add(-time.Hour)

	p.GrantLifetime(TierPro, "life-1", purchased)
	p.GrantLifetime(TierPro, "life-1", purchased)

	if p.Tier != TierPro || p.LifetimeTier != TierPro {
		t.Errorf("Tier/LifetimeTier = %v/%v, want Pro/Pro", p.Tier, p.LifetimeTier)
	}
	if p.LifetimeOriginalTransactionID == nil || *p.LifetimeOriginalTransactionID != "life-1" {
		t.Errorf("LifetimeOriginalTransactionID = %v, want life-1", p.LifetimeOriginalTransactionID)
	}
	if p.LifetimePurchasedAt == nil || !p.LifetimePurchasedAt.Equal(purchased) {
		t.Errorf("LifetimePurchasedAt = %v, want %v", p.LifetimePurchasedAt, purchased)
	}
}

func TestProfile_ExpireSubscription_KeepsLifetimeFloor(t *testing.T) {
	t.Parallel()
	p := lifetimePro(t)
	expiry := lifetimeNow.AddDate(0, 1, 0)
	grace := lifetimeNow.AddDate(0, 0, 16)
	product := "uk.towncrierapp.pro.monthly"
	p.SubscriptionExpiry = &expiry
	p.GracePeriodExpiry = &grace
	p.SubscriptionProductID = &product

	p.ExpireSubscription()

	if p.Tier != TierPro {
		t.Errorf("Tier = %v, want Pro (lifetime floor)", p.Tier)
	}
	if p.SubscriptionExpiry != nil || p.GracePeriodExpiry != nil || p.SubscriptionProductID != nil {
		t.Errorf("expiry/grace/product = %v/%v/%v, want all nil", p.SubscriptionExpiry, p.GracePeriodExpiry, p.SubscriptionProductID)
	}
}

func TestProfile_ExpireSubscription_WithoutLifetimeDropsToFree(t *testing.T) {
	t.Parallel()
	expiry := lifetimeNow.AddDate(0, 1, 0)
	p := &UserProfile{Tier: TierPro, SubscriptionExpiry: &expiry}

	p.ExpireSubscription()

	if p.Tier != TierFree {
		t.Errorf("Tier = %v, want Free", p.Tier)
	}
}

func TestProfile_ActivateSubscription_NeverBelowLifetime(t *testing.T) {
	t.Parallel()
	p := lifetimePro(t)
	product := "uk.towncrierapp.pro.annual"
	p.SubscriptionProductID = &product

	p.ActivateSubscription(TierPersonal, lifetimeNow.AddDate(0, 1, 0))

	if p.Tier != TierPro {
		t.Errorf("Tier = %v, want Pro (lifetime floor)", p.Tier)
	}
	if p.SubscriptionExpiry == nil {
		t.Error("SubscriptionExpiry = nil, want set")
	}
	if p.SubscriptionProductID != nil {
		t.Errorf("SubscriptionProductID = %v, want nil", *p.SubscriptionProductID)
	}
}

func TestProfile_ActivateAppStoreSubscription_RecordsProductID(t *testing.T) {
	t.Parallel()
	p := &UserProfile{Tier: TierFree}
	expiry := lifetimeNow.AddDate(1, 0, 0)

	p.ActivateAppStoreSubscription(TierPro, expiry, "uk.towncrierapp.pro.annual")

	if p.Tier != TierPro {
		t.Errorf("Tier = %v, want Pro", p.Tier)
	}
	if p.SubscriptionExpiry == nil || !p.SubscriptionExpiry.Equal(expiry) {
		t.Errorf("SubscriptionExpiry = %v, want %v", p.SubscriptionExpiry, expiry)
	}
	if p.SubscriptionProductID == nil || *p.SubscriptionProductID != "uk.towncrierapp.pro.annual" {
		t.Errorf("SubscriptionProductID = %v, want uk.towncrierapp.pro.annual", p.SubscriptionProductID)
	}
}

func TestProfile_RevokeLifetime_LifetimeOnlyDropsToFree(t *testing.T) {
	t.Parallel()
	p := lifetimePro(t)

	p.RevokeLifetime()

	if p.Tier != TierFree {
		t.Errorf("Tier = %v, want Free", p.Tier)
	}
	if p.LifetimeTier != TierFree {
		t.Errorf("LifetimeTier = %v, want Free", p.LifetimeTier)
	}
	if p.LifetimeOriginalTransactionID != nil || p.LifetimePurchasedAt != nil {
		t.Errorf("lifetime fields = %v/%v, want nil", p.LifetimeOriginalTransactionID, p.LifetimePurchasedAt)
	}
}

func TestProfile_RevokeLifetime_OpenSubscriptionKeepsTier(t *testing.T) {
	t.Parallel()
	p := lifetimePro(t)
	expiry := lifetimeNow.AddDate(0, 1, 0)
	p.SubscriptionExpiry = &expiry

	p.RevokeLifetime()

	if p.Tier != TierPro {
		t.Errorf("Tier = %v, want Pro (subscription window still open)", p.Tier)
	}
	if p.LifetimeTier != TierFree {
		t.Errorf("LifetimeTier = %v, want Free", p.LifetimeTier)
	}
	if p.LifetimeOriginalTransactionID != nil || p.LifetimePurchasedAt != nil {
		t.Errorf("lifetime fields = %v/%v, want nil", p.LifetimeOriginalTransactionID, p.LifetimePurchasedAt)
	}
}

func TestProfile_EffectiveTier_LifetimeWithNilExpiry(t *testing.T) {
	t.Parallel()
	p := lifetimePro(t)

	if got := p.EffectiveTier(lifetimeNow); got != TierPro {
		t.Errorf("EffectiveTier() = %v, want Pro", got)
	}
}

func TestProfile_EffectiveTier_LifetimeOutlivesLapsedSubscription(t *testing.T) {
	t.Parallel()
	p := lifetimePro(t)
	past := lifetimeNow.Add(-time.Hour)
	p.Tier = TierPersonal
	p.SubscriptionExpiry = &past

	if got := p.EffectiveTier(lifetimeNow); got != TierPro {
		t.Errorf("EffectiveTier() = %v, want Pro (lifetime outlives lapsed Personal)", got)
	}
}
