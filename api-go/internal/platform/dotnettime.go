package platform

import (
	"encoding/json"
	"fmt"
	"time"
)

// DotNetTime marshals as ISO 8601 with a numeric UTC offset (e.g.
// "2026-06-12T09:30:00+00:00"), never Go's RFC 3339 "Z" suffix; fractional
// seconds appear only when non-zero, trailing zeros trimmed.
type DotNetTime time.Time

// String renders the instant as ISO 8601 with a numeric UTC offset.
func (t DotNetTime) String() string {
	return time.Time(t).Format("2006-01-02T15:04:05.9999999-07:00")
}

// MarshalJSON renders the instant as ISO 8601 with a numeric UTC offset.
func (t DotNetTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.String() + `"`), nil
}

// UnmarshalJSON accepts both the "+00:00" offset form this type writes and the
// RFC 3339 "Z" form.
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
