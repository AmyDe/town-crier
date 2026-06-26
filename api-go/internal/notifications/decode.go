package notifications

import (
	"encoding/json"
	"fmt"
)

// DecodeDocument hydrates a stored Notifications-container document body into
// a DigestNotification, reusing the exact field mapping the Cosmos read path
// uses (digestDocument.toDigest). It exists so the Cosmos → Postgres
// notifications backfill (cmd/pgbackfill-notifications) shares one transform
// with the store rather than reinventing the mapping, keeping the two from
// silently diverging.
//
// Legacy rows (predating the eventType / applicationUid fields) are coalesced
// exactly as the store read path does: a null eventType becomes NewApplication
// and a null applicationUid falls back to applicationName. A row that fails
// JSON decode is returned as an error; structural coalescing is not an error.
func DecodeDocument(raw []byte) (DigestNotification, error) {
	var doc digestDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return DigestNotification{}, fmt.Errorf("decode notification document: %w", err)
	}
	return doc.toDigest(), nil
}
