package subscriptions

import (
	"errors"
	"strings"
	"testing"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

func TestTierForProduct(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		productID string
		want      profiles.SubscriptionTier
		wantErr   bool
	}{
		{"personal monthly", "uk.towncrierapp.personal.monthly", profiles.TierPersonal, false},
		{"pro monthly", "uk.towncrierapp.pro.monthly", profiles.TierPro, false},
		// The legacy typo IDs (extra ".co.") must NOT map — they are the bug this
		// mapping deliberately does not carry over (tc-7g3i.12).
		{"legacy personal typo", "uk.co.towncrier.personal.monthly", profiles.TierFree, true},
		{"legacy pro typo", "uk.co.towncrier.pro.monthly", profiles.TierFree, true},
		{"unknown", "com.example.bogus", profiles.TierFree, true},
		{"empty", "", profiles.TierFree, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := TierForProduct(tc.productID)
			if (err != nil) != tc.wantErr {
				t.Fatalf("TierForProduct(%q) err=%v, wantErr=%v", tc.productID, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("TierForProduct(%q) = %v, want %v", tc.productID, got, tc.want)
			}
		})
	}
}

func TestTierForProduct_UnknownErrorMessage(t *testing.T) {
	t.Parallel()
	_, err := TierForProduct("com.example.bogus")
	var upe *UnknownProductError
	if !errors.As(err, &upe) {
		t.Fatalf("want *UnknownProductError, got %T (%v)", err, err)
	}
	// Message matches .NET ProductMapping's ArgumentException text so a future
	// contract test could diff it against the retired .NET API verbatim.
	if want := "Unknown App Store product ID: 'com.example.bogus'"; upe.Error() != want {
		t.Errorf("message = %q, want %q", upe.Error(), want)
	}
}

// TestProductIDs_NoLegacyDomain guards against anyone reintroducing the .NET
// "uk.co.towncrier" domain typo into the canonical constants.
func TestProductIDs_NoLegacyDomain(t *testing.T) {
	t.Parallel()
	for _, id := range []string{ProductPersonalMonthly, ProductProMonthly} {
		if strings.Contains(id, "uk.co.towncrier") {
			t.Errorf("product id %q contains the legacy uk.co.towncrier typo", id)
		}
		if !strings.HasPrefix(id, "uk.towncrierapp.") {
			t.Errorf("product id %q is not under the canonical uk.towncrierapp. prefix", id)
		}
	}
}
