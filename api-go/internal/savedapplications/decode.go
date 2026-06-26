package savedapplications

import (
	"encoding/json"
	"fmt"
)

// DecodeDocument hydrates a stored SavedApplications-container document body into
// a domain SavedApplication, reusing the exact field mapping the Cosmos read path
// uses (savedApplicationDocument.toDomain). It exists so the Cosmos → Postgres
// saved-application backfill (cmd/pgbackfill-saved) shares one transform with
// the store rather than reinventing the mapping, which keeps the two from
// silently diverging.
//
// authorityID coalesces in toDomain(): the document's authorityId field takes
// precedence; when absent the embedded snapshot's areaId fills in, exactly as the
// store read path does.
func DecodeDocument(raw []byte) (SavedApplication, error) {
	var doc savedApplicationDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return SavedApplication{}, fmt.Errorf("decode saved application document: %w", err)
	}
	return doc.toDomain(), nil
}
