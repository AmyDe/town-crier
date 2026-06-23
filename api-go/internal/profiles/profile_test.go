package profiles

import (
	"testing"
	"time"
)

func TestNewProfile_Defaults(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	p, err := NewProfile("auth0|abc", "user@example.com", now)
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}

	if p.UserID != "auth0|abc" {
		t.Errorf("UserID: got %q", p.UserID)
	}
	if p.Email == nil || *p.Email != "user@example.com" {
		t.Errorf("Email: got %v, want user@example.com", p.Email)
	}
	if p.Tier != TierFree {
		t.Errorf("Tier: got %v, want Free", p.Tier)
	}
	// Default notification preferences: push on, Monday digest, email digest
	// + saved-decision channels all on.
	if !p.Preferences.PushEnabled {
		t.Error("PushEnabled: got false, want true (default)")
	}
	if p.Preferences.DigestDay != time.Monday {
		t.Errorf("DigestDay: got %v, want Monday", p.Preferences.DigestDay)
	}
	if !p.Preferences.EmailDigestEnabled || !p.Preferences.SavedDecisionPush || !p.Preferences.SavedDecisionEmail {
		t.Error("default email/saved-decision flags should all be true")
	}
	if !p.LastActiveAt.Equal(now) {
		t.Errorf("LastActiveAt: got %v, want %v", p.LastActiveAt, now)
	}
}

func TestNewProfile_RejectsEmptyUserID(t *testing.T) {
	t.Parallel()

	tests := []string{"", "   "}
	for _, userID := range tests {
		if _, err := NewProfile(userID, "", time.Now()); err == nil {
			t.Errorf("NewProfile(%q): want error, got nil", userID)
		}
	}
}

func TestNewProfile_NilEmailWhenBlank(t *testing.T) {
	t.Parallel()

	p, err := NewProfile("auth0|abc", "", time.Now())
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	if p.Email != nil {
		t.Errorf("Email: got %v, want nil for blank email", p.Email)
	}
}

func TestProfile_RecordActivity_OnlyAdvances(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	p, _ := NewProfile("u1", "", base)

	// Earlier time is ignored (LastActiveAt only moves forward).
	p.RecordActivity(base.Add(-time.Hour))
	if !p.LastActiveAt.Equal(base) {
		t.Errorf("RecordActivity backwards moved LastActiveAt to %v", p.LastActiveAt)
	}

	later := base.Add(48 * time.Hour)
	p.RecordActivity(later)
	if !p.LastActiveAt.Equal(later) {
		t.Errorf("RecordActivity: got %v, want %v", p.LastActiveAt, later)
	}
}

func TestProfile_BackfillEmail(t *testing.T) {
	t.Parallel()

	p, _ := NewProfile("u1", "", time.Now())
	p.BackfillEmail("new@example.com")
	if p.Email == nil || *p.Email != "new@example.com" {
		t.Errorf("BackfillEmail: got %v, want new@example.com", p.Email)
	}

	// Backfill must not overwrite an existing email.
	p.BackfillEmail("other@example.com")
	if *p.Email != "new@example.com" {
		t.Errorf("BackfillEmail overwrote existing email: got %v", *p.Email)
	}
}

func TestProfile_ActivateSubscription(t *testing.T) {
	t.Parallel()

	p, _ := NewProfile("u1", "", time.Now())
	expiry := time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)
	p.ActivateSubscription(TierPro, expiry)

	if p.Tier != TierPro {
		t.Errorf("Tier: got %v, want Pro", p.Tier)
	}
	if p.SubscriptionExpiry == nil || !p.SubscriptionExpiry.Equal(expiry) {
		t.Errorf("SubscriptionExpiry: got %v, want %v", p.SubscriptionExpiry, expiry)
	}
}

func TestSubscriptionTier_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tier SubscriptionTier
		want string
	}{
		{TierFree, "Free"},
		{TierPersonal, "Personal"},
		{TierPro, "Pro"},
	}
	for _, tc := range tests {
		if got := tc.tier.String(); got != tc.want {
			t.Errorf("%v.String(): got %q, want %q", tc.tier, got, tc.want)
		}
	}
}

func TestParseSubscriptionTier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      string
		want    SubscriptionTier
		wantErr bool
	}{
		{"Free", TierFree, false},
		{"Personal", TierPersonal, false},
		{"Pro", TierPro, false},
		{"bogus", TierFree, true},
		{"", TierFree, true},
	}
	for _, tc := range tests {
		got, err := ParseSubscriptionTier(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseSubscriptionTier(%q): err=%v wantErr=%v", tc.in, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("ParseSubscriptionTier(%q): got %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSubscriptionTier_IsPaid(t *testing.T) {
	t.Parallel()

	if TierFree.IsPaid() {
		t.Error("Free should not be paid")
	}
	if !TierPersonal.IsPaid() || !TierPro.IsPaid() {
		t.Error("Personal and Pro should be paid")
	}
}

func TestUserProfile_EffectiveTier(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	ptr := func(tm time.Time) *time.Time { return &tm }
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	farFuture := time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		tier   SubscriptionTier
		expiry *time.Time
		grace  *time.Time
		want   SubscriptionTier
	}{
		{
			// Free always stays Free regardless of any stale expiry/grace fields.
			name: "free", tier: TierFree, expiry: ptr(past), grace: ptr(past), want: TierFree,
		},
		{
			// Paid tier still inside its window is returned unchanged.
			name: "paid within window", tier: TierPro, expiry: ptr(future), grace: nil, want: TierPro,
		},
		{
			// Lapsed paid tier with no grace collapses to Free (the offer-code gap).
			name: "paid expired no grace", tier: TierPro, expiry: ptr(past), grace: nil, want: TierFree,
		},
		{
			// Lapsed paid tier whose grace period is still live keeps the tier.
			name: "paid expired grace live", tier: TierPersonal, expiry: ptr(past), grace: ptr(future), want: TierPersonal,
		},
		{
			// Lapsed paid tier whose grace period has also passed collapses to Free.
			name: "paid expired grace past", tier: TierPersonal, expiry: ptr(past), grace: ptr(past), want: TierFree,
		},
		{
			// A paid tier with no expiry is malformed; treated as entitled rather
			// than silently downgraded (no proof of expiry).
			name: "nil expiry paid", tier: TierPro, expiry: nil, grace: nil, want: TierPro,
		},
		{
			// Pro-domain auto-grants use a far-future expiry, so they are naturally
			// exempt — no special-casing needed.
			name: "far future pro-domain grant", tier: TierPro, expiry: ptr(farFuture), grace: nil, want: TierPro,
		},
		{
			// Boundary: expiry exactly at now counts as expired (mirrors the
			// lapsed-txn filter: expired when NOT strictly after now).
			name: "boundary expiry equals now", tier: TierPro, expiry: ptr(now), grace: nil, want: TierFree,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &UserProfile{
				Tier:               tc.tier,
				SubscriptionExpiry: tc.expiry,
				GracePeriodExpiry:  tc.grace,
			}
			if got := p.EffectiveTier(now); got != tc.want {
				t.Errorf("EffectiveTier() = %v, want %v", got, tc.want)
			}
		})
	}
}
