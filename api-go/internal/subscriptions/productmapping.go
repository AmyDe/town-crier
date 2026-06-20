// Package subscriptions owns the Apple StoreKit subscription feature: the
// canonical product-ID -> tier mapping, the hand-rolled JWS verifier (ES256,
// x5c chain to the embedded Apple Root CA - G3), the transaction and
// notification decoders, the Cosmos notification-idempotency store, and the
// POST /v1/subscriptions/verify (authed) + POST /v1/webhooks/appstore
// (anonymous) handlers (GH#418 iteration 9).
package subscriptions

import (
	"fmt"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// Canonical App Store Connect product IDs. These are the strings the iOS app
// requests and App Store Connect issues — under the uk.towncrierapp prefix,
// matching the app bundle id uk.towncrierapp.mobile and the domain
// towncrierapp.uk.
//
// Legacy product IDs uk.co.towncrier.personal.monthly /
// uk.co.towncrier.pro.monthly (an extra ".co.", wrong domain) were never correct
// and never matched a real purchase. They are deliberately NOT added here
// (tc-7g3i.12).
const (
	// ProductPersonalMonthly is the Personal-tier monthly subscription.
	ProductPersonalMonthly = "uk.towncrierapp.personal.monthly"
	// ProductProMonthly is the Pro-tier monthly subscription.
	ProductProMonthly = "uk.towncrierapp.pro.monthly"
)

// UnknownProductError signals a product ID with no tier mapping. Its message is
// surfaced verbatim in the 400 invalid_transaction_payload response body.
type UnknownProductError struct {
	ProductID string
}

func (e *UnknownProductError) Error() string {
	return fmt.Sprintf("Unknown App Store product ID: '%s'", e.ProductID)
}

// TierForProduct maps a canonical App Store product ID to its subscription
// tier. An unrecognised ID — including the legacy typo IDs — yields an
// *UnknownProductError and the Free tier.
func TierForProduct(productID string) (profiles.SubscriptionTier, error) {
	switch productID {
	case ProductPersonalMonthly:
		return profiles.TierPersonal, nil
	case ProductProMonthly:
		return profiles.TierPro, nil
	default:
		return profiles.TierFree, &UnknownProductError{ProductID: productID}
	}
}
