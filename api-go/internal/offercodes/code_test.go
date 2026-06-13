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
