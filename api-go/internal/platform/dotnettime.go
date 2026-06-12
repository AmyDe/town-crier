package platform

import (
	"encoding/json"
	"fmt"
	"time"
)

// DotNetTime marshals like System.Text.Json's DateTimeOffset: ISO 8601 with a
// numeric UTC offset ("2099-12-31T00:00:00+00:00"), never Go's RFC 3339 "Z"
// suffix; fractional seconds appear only when non-zero, trailing zeros trimmed —
// the same trimming STJ applies. Cosmos documents and wire responses that carry
// a .NET DateTimeOffset (device RegisteredAt, notification-state LastReadAt) must
// use this type so the contract-diff harness sees byte-identical timestamps.
type DotNetTime time.Time

// String renders the instant in the .NET DateTimeOffset wire format. Exposed
// for Cosmos query parameters, where the comparison against stored
// "+00:00"-formatted strings is lexicographic and must use the same layout.
func (t DotNetTime) String() string {
	return time.Time(t).Format("2006-01-02T15:04:05.9999999-07:00")
}

// MarshalJSON renders the instant in the .NET DateTimeOffset wire format.
func (t DotNetTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.String() + `"`), nil
}

// UnmarshalJSON parses a stored timestamp, accepting both the .NET "+00:00"
// offset form this type writes and the RFC 3339 "Z" form, so a Cosmos document
// written by either implementation hydrates correctly.
func (t *DotNetTime) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return fmt.Errorf("parse dotnet time %q: %w", s, err)
	}
	*t = DotNetTime(parsed)
	return nil
}

// DotNetTimePtr converts an optional time, preserving nil so an absent value
// serialises as null rather than a zero instant.
func DotNetTimePtr(t *time.Time) *DotNetTime {
	if t == nil {
		return nil
	}
	dt := DotNetTime(*t)
	return &dt
}
