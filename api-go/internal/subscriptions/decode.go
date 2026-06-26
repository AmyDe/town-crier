package subscriptions

import (
	"encoding/json"
	"fmt"
	"time"
)

// ProcessedNotification is the domain record for a processed App Store Server
// Notification, as decoded from a stored AppleNotifications-container document.
type ProcessedNotification struct {
	UUID        string
	ProcessedAt time.Time
}

// DecodeDocument hydrates a stored AppleNotifications-container document body
// into a ProcessedNotification domain record, reusing the exact field mapping
// the Cosmos store uses (processedNotificationDocument). It exists so the
// Cosmos -> Postgres backfill (cmd/pgbackfill-applenotifs) shares one
// transform with the store rather than reinventing the mapping, which keeps
// the two from silently diverging. A document with an empty id is rejected
// with an error.
func DecodeDocument(raw []byte) (ProcessedNotification, error) {
	var doc processedNotificationDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ProcessedNotification{}, fmt.Errorf("decode apple notification document: %w", err)
	}
	if doc.ID == "" {
		return ProcessedNotification{}, fmt.Errorf("apple notification document has empty id")
	}
	return ProcessedNotification{
		UUID:        doc.ID,
		ProcessedAt: time.Time(doc.ProcessedAt),
	}, nil
}
