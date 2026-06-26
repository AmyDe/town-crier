package profiles

import (
	"encoding/json"
	"fmt"
)

// DecodeDocument hydrates a stored Users-container document body into a domain
// UserProfile, reusing the exact field mapping the Cosmos read path uses
// (profileDocument.toDomain). It exists so the Cosmos → Postgres users backfill
// (cmd/pgbackfill-users) shares one transform with the store rather than
// reinventing the mapping, which keeps the two from silently diverging.
//
// The nullable preference flags (emailDigestEnabled / savedDecision*) coalesce
// to true when absent — the opt-in default for documents written before these
// fields existed. An unrecognised tier string is rejected with an error rather
// than silently defaulted.
func DecodeDocument(raw []byte) (*UserProfile, error) {
	var doc profileDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("decode profile document: %w", err)
	}
	return doc.toDomain()
}
