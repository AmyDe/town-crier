package applications

import (
	"encoding/json"
	"fmt"
)

// DecodeDocument hydrates a stored Applications document body into a domain
// PlanningApplication. A document with an absent or malformed location (fewer
// than two coordinates) decodes to nil Longitude/Latitude rather than an error;
// an explicit [0,0] point is preserved.
func DecodeDocument(raw []byte) (PlanningApplication, error) {
	var doc applicationDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return PlanningApplication{}, fmt.Errorf("decode application document: %w", err)
	}
	return doc.toDomain(), nil
}
