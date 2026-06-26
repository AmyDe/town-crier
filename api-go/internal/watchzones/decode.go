package watchzones

import (
	"encoding/json"
	"fmt"
)

// DecodeDocument hydrates a stored WatchZones-container document body into a
// domain WatchZone, reusing the exact field mapping the Cosmos read path uses
// (watchZoneDocument.toDomain). It exists so the Cosmos -> Postgres watch-zone
// backfill (cmd/pgbackfill-zones) shares one transform with the store rather
// than reinventing the mapping, which keeps the two from silently diverging.
//
// The nullable flags (pushEnabled / emailInstantEnabled) coalesce to true when
// absent and an absent createdAt hydrates to the zero instant, exactly as the
// store read path does. A document whose values violate a WatchZone invariant
// (blank id/user/name, non-positive radius or authority id) is rejected with the
// constructor's error rather than silently carried.
func DecodeDocument(raw []byte) (WatchZone, error) {
	var doc watchZoneDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return WatchZone{}, fmt.Errorf("decode watch zone document: %w", err)
	}
	return doc.toDomain()
}
