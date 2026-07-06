package offercodes

import (
	"errors"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

func TestNewOfferCode_Validates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name           string
		code           string
		tier           profiles.SubscriptionTier
		duration       int
		label          string
		maxRedemptions int
		wantErr        bool
	}{
		{"valid", "ABCDEFGHJKMN", profiles.TierPro, 30, "creator-campaign", 1, false},
		{"non-canonical code", "ABC", profiles.TierPro, 30, "label", 1, true},
		{"free tier rejected", "ABCDEFGHJKMN", profiles.TierFree, 30, "label", 1, true},
		{"duration too low", "ABCDEFGHJKMN", profiles.TierPro, 0, "label", 1, true},
		{"duration too high", "ABCDEFGHJKMN", profiles.TierPro, 366, "label", 1, true},
		{"blank label rejected", "ABCDEFGHJKMN", profiles.TierPro, 30, "", 1, true},
		{"whitespace-only label rejected", "ABCDEFGHJKMN", profiles.TierPro, 30, "   ", 1, true},
		{"label at max length accepted", "ABCDEFGHJKMN", profiles.TierPro, 30, repeatChar('a', 100), 1, false},
		{"label over max length rejected", "ABCDEFGHJKMN", profiles.TierPro, 30, repeatChar('a', 101), 1, true},
		{"maxRedemptions zero rejected", "ABCDEFGHJKMN", profiles.TierPro, 30, "label", 0, true},
		{"maxRedemptions at upper bound accepted", "ABCDEFGHJKMN", profiles.TierPro, 30, "label", 10000, false},
		{"maxRedemptions over upper bound rejected", "ABCDEFGHJKMN", profiles.TierPro, 30, "label", 10001, true},
		{"maxRedemptions negative rejected", "ABCDEFGHJKMN", profiles.TierPro, 30, "label", -1, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewOfferCode(tc.code, tc.tier, tc.duration, tc.label, tc.maxRedemptions, now)
			if (err != nil) != tc.wantErr {
				t.Fatalf("NewOfferCode err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestNewOfferCode_TrimsLabel(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	code, err := NewOfferCode("ABCDEFGHJKMN", profiles.TierPro, 30, "  creator-campaign  ", 1, now)
	if err != nil {
		t.Fatalf("NewOfferCode: %v", err)
	}
	if code.Label != "creator-campaign" {
		t.Errorf("Label = %q, want trimmed %q", code.Label, "creator-campaign")
	}
}

func repeatChar(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

func TestOfferCode_IsFullyRedeemed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		redemptionCount int
		maxRedemptions  int
		want            bool
	}{
		{"fresh code with cap 1", 0, 1, false},
		{"single-use consumed", 1, 1, true},
		{"multi-use partially consumed", 2, 3, false},
		{"multi-use fully consumed", 3, 3, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := OfferCode{
				Code:            "ABCDEFGHJKMN",
				Tier:            profiles.TierPro,
				DurationDays:    30,
				MaxRedemptions:  tc.maxRedemptions,
				RedemptionCount: tc.redemptionCount,
			}
			if got := c.IsFullyRedeemed(); got != tc.want {
				t.Errorf("IsFullyRedeemed() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestRedemption_ActiveAt is the single authoritative "still-active offer
// window" rule reused by both the list-users row column and the admin stats
// aggregate: a redemption is active only while its own redeemed_at + duration
// window has not yet closed. A nil RedeemedAt (never redeemed, or scrubbed by
// GDPR anonymisation) is never active.
func TestRedemption_ActiveAt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	redeemedRecently := now.Add(-10 * 24 * time.Hour)
	redeemedLongAgo := now.Add(-40 * 24 * time.Hour)
	redeemedExactlyAtBoundary := now.Add(-30 * 24 * time.Hour)

	tests := []struct {
		name         string
		durationDays int
		redeemedAt   *time.Time
		want         bool
	}{
		{"never redeemed (nil)", 30, nil, false},
		{"within window", 30, &redeemedRecently, true},
		{"past window", 30, &redeemedLongAgo, false},
		{"exactly at boundary is expired", 30, &redeemedExactlyAtBoundary, false},
		{"zero duration is never active", 0, &redeemedRecently, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := Redemption{Code: "ABCDEFGHJKMN", RedeemedAt: tc.redeemedAt}
			if got := r.ActiveAt(now, tc.durationDays); got != tc.want {
				t.Errorf("ActiveAt(%v, %d) = %v, want %v", now, tc.durationDays, got, tc.want)
			}
		})
	}
}

// TestRedeemedOfferCode_ActiveAt confirms the join-result type delegates to
// Redemption.ActiveAt using its own DurationDays, so admin/users.go and the
// GDPR export never diverge from the single authoritative rule.
func TestRedeemedOfferCode_ActiveAt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	redeemedRecently := now.Add(-10 * 24 * time.Hour)

	active := RedeemedOfferCode{Code: "AAAAAAAAAAAA", Tier: profiles.TierPro, DurationDays: 30, RedeemedAt: &redeemedRecently}
	if !active.ActiveAt(now) {
		t.Error("expected active redemption to report ActiveAt = true")
	}

	anonymised := RedeemedOfferCode{Code: "BBBBBBBBBBBB", Tier: profiles.TierPro, DurationDays: 30, RedeemedAt: nil}
	if anonymised.ActiveAt(now) {
		t.Error("an anonymised redemption (nil RedeemedAt) must never be active")
	}
}

func TestErrAlreadyRedeemedByUser_IsDistinctFromErrAlreadyRedeemed(t *testing.T) {
	t.Parallel()
	if errors.Is(ErrAlreadyRedeemedByUser, ErrAlreadyRedeemed) {
		t.Error("ErrAlreadyRedeemedByUser must be a distinct sentinel from ErrAlreadyRedeemed")
	}
}
