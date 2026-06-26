package devicetokens

import (
	"encoding/json"
	"fmt"
)

// DecodeDocument hydrates a stored DeviceRegistrations-container document body
// into a domain DeviceRegistration, reusing the exact field mapping the Cosmos
// read path uses (deviceDocument.toDomain). It exists so the Cosmos → Postgres
// device-token backfill (cmd/pgbackfill-devices) shares one transform with the
// store rather than reinventing the mapping, which keeps the two from silently
// diverging.
func DecodeDocument(raw []byte) (DeviceRegistration, error) {
	var doc deviceDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return DeviceRegistration{}, fmt.Errorf("decode device token document: %w", err)
	}
	return doc.toDomain()
}
