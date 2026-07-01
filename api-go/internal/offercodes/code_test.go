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
		name     string
		code     string
		tier     profiles.SubscriptionTier
		duration int
		wantErr  bool
	}{
		{"valid", "ABCDEFGHJKMN", profiles.TierPro, 30, false},
		{"non-canonical code", "ABC", profiles.TierPro, 30, true},
		{"free tier rejected", "ABCDEFGHJKMN", profiles.TierFree, 30, true},
		{"duration too low", "ABCDEFGHJKMN", profiles.TierPro, 0, true},
		{"duration too high", "ABCDEFGHJKMN", profiles.TierPro, 366, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewOfferCode(tc.code, tc.tier, tc.duration, now)
			if (err != nil) != tc.wantErr {
				t.Fatalf("NewOfferCode err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestOfferCode_Redeem(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	code, err := NewOfferCode("ABCDEFGHJKMN", profiles.TierPro, 30, now)
	if err != nil {
		t.Fatalf("NewOfferCode: %v", err)
	}
	if code.IsRedeemed() {
		t.Fatal("new code should not be redeemed")
	}

	if err := code.Redeem("auth0|u1", now); err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	if !code.IsRedeemed() {
		t.Error("code should be redeemed after Redeem")
	}
	if code.RedeemedByUserID == nil || *code.RedeemedByUserID != "auth0|u1" {
		t.Errorf("RedeemedByUserID = %v, want auth0|u1", code.RedeemedByUserID)
	}
	if code.RedeemedAt == nil || !code.RedeemedAt.Equal(now) {
		t.Errorf("RedeemedAt = %v, want %v", code.RedeemedAt, now)
	}
}

func TestOfferCode_Redeem_RejectsSecondRedemption(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	code, _ := NewOfferCode("ABCDEFGHJKMN", profiles.TierPro, 30, now)
	_ = code.Redeem("auth0|u1", now)

	if err := code.Redeem("auth0|u2", now); !errors.Is(err, ErrAlreadyRedeemed) {
		t.Errorf("second Redeem err = %v, want ErrAlreadyRedeemed", err)
	}
}

// ActiveAt is the single authoritative "still-active offer window" rule reused
// by both the list-users row column and the admin stats aggregate: a code is
// active only while its own redeemed_at + duration window has not yet closed.
func TestOfferCode_ActiveAt(t *testing.T) {
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
		{"never redeemed", 30, nil, false},
		{"within window", 30, &redeemedRecently, true},
		{"past window", 30, &redeemedLongAgo, false},
		{"exactly at boundary is expired", 30, &redeemedExactlyAtBoundary, false},
		{"zero duration is never active", 0, &redeemedRecently, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := OfferCode{
				Code:         "ABCDEFGHJKMN",
				Tier:         profiles.TierPro,
				DurationDays: tc.durationDays,
				RedeemedAt:   tc.redeemedAt,
			}
			if got := c.ActiveAt(now); got != tc.want {
				t.Errorf("ActiveAt(%v) = %v, want %v", now, got, tc.want)
			}
		})
	}
}

// An anonymised code (redeemer scrubbed for GDPR Art. 17, but the consumed
// tombstone retained) must still report as redeemed and reject re-redemption,
// even though its RedeemedByUserID / RedeemedAt are now nil.
func TestOfferCode_Anonymised_StaysRedeemed(t *testing.T) {
	t.Parallel()

	code := OfferCode{
		Code:             "ABCDEFGHJKMN",
		Tier:             profiles.TierPro,
		DurationDays:     30,
		CreatedAt:        time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Redeemed:         true,
		RedeemedByUserID: nil,
		RedeemedAt:       nil,
	}

	if !code.IsRedeemed() {
		t.Error("anonymised code with the consumed tombstone should still be redeemed")
	}
	if err := code.Redeem("auth0|u2", time.Now()); !errors.Is(err, ErrAlreadyRedeemed) {
		t.Errorf("re-redeem of anonymised code err = %v, want ErrAlreadyRedeemed", err)
	}
}
