package offercodes

import (
	"encoding/json"
	"fmt"
)

// DecodeDocument decodes a raw Cosmos OfferCodes-container document into a
// domain OfferCode, applying the same legacy-coalesce rule as the Cosmos store:
// a document with redeemed=false but a non-nil redeemedByUserId is treated as
// redeemed (older data written before the redeemed boolean column was added).
//
// It is the shared decode path used by the pgbackfill-offercodes backfill tool.
func DecodeDocument(raw []byte) (OfferCode, error) {
	var doc offerCodeDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return OfferCode{}, fmt.Errorf("decode offer code document: %w", err)
	}
	code, err := doc.toDomain()
	if err != nil {
		return OfferCode{}, fmt.Errorf("hydrate offer code document: %w", err)
	}
	return code, nil
}
