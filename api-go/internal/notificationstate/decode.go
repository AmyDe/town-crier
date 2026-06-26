package notificationstate

import (
	"encoding/json"
	"fmt"
)

// DecodeDocument hydrates a stored NotificationState-container document body
// into a State, reusing the exact field mapping the Cosmos read path uses
// (stateDocument.toDomain). It exists so the Cosmos → Postgres notification-
// state backfill (cmd/pgbackfill-notifstate) shares one transform with the
// store rather than reinventing the mapping, keeping the two from silently
// diverging.
func DecodeDocument(raw []byte) (State, error) {
	var doc stateDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return State{}, fmt.Errorf("decode notification state document: %w", err)
	}
	return doc.toDomain(), nil
}
