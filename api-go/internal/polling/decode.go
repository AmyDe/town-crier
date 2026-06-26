package polling

import (
	"encoding/json"
	"fmt"
)

// DecodeDocument hydrates a stored PollState-container document body into an
// authority id and domain PollState, reusing the exact field mapping the Cosmos
// read path uses (pollStateDocument.toState). It exists so the Cosmos ->
// Postgres poll-state backfill (cmd/pgbackfill-pollstate) shares one transform
// with the store rather than reinventing the mapping.
//
// A document whose LastPollTime or HighWaterMark cannot be parsed, or whose
// cursor fields are partially present, is rejected with a descriptive error
// rather than silently carried.
func DecodeDocument(raw []byte) (authorityID int, state PollState, err error) {
	var doc pollStateDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return 0, PollState{}, fmt.Errorf("decode poll state document: %w", err)
	}
	s, err := doc.toState()
	if err != nil {
		return 0, PollState{}, fmt.Errorf("hydrate poll state document authority %d: %w", doc.AuthorityID, err)
	}
	return doc.AuthorityID, s, nil
}
