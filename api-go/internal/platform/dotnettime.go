package platform

import "time"

// DotNetTime marshals like System.Text.Json's DateTimeOffset: ISO 8601 with a
// numeric UTC offset ("2099-12-31T00:00:00+00:00"), never Go's RFC 3339 "Z"
// suffix; fractional seconds appear only when non-zero, trailing zeros trimmed —
// the same trimming STJ applies. Cosmos documents and wire responses that carry
// a .NET DateTimeOffset (device RegisteredAt, notification-state LastReadAt) must
// use this type so the contract-diff harness sees byte-identical timestamps.
type DotNetTime time.Time

// MarshalJSON renders the instant in the .NET DateTimeOffset wire format.
func (t DotNetTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(t).Format("2006-01-02T15:04:05.9999999-07:00") + `"`), nil
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
