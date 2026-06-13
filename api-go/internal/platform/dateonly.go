package platform

import (
	"encoding/json"
	"fmt"
	"time"
)

// dateOnlyLayout is the wire format of a .NET DateOnly: an ISO calendar date
// with no time component, e.g. "2026-03-07".
const dateOnlyLayout = "2006-01-02"

// DateOnly marshals like System.Text.Json's DateOnly: a bare "yyyy-MM-dd" date,
// never a full timestamp. Planning-application dates (start/decided/consulted)
// carry .NET DateOnly values, so wire and stored documents must use this type to
// stay byte-identical under the contract-diff harness.
type DateOnly time.Time

// String renders the calendar date in the .NET DateOnly wire format.
func (d DateOnly) String() string {
	return time.Time(d).Format(dateOnlyLayout)
}

// MarshalJSON renders the calendar date as a quoted "yyyy-MM-dd" string.
func (d DateOnly) MarshalJSON() ([]byte, error) {
	return []byte(`"` + d.String() + `"`), nil
}

// UnmarshalJSON parses a bare "yyyy-MM-dd" date into the zero-time-of-day
// instant in UTC.
func (d *DateOnly) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := time.Parse(dateOnlyLayout, s)
	if err != nil {
		return fmt.Errorf("parse date-only %q: %w", s, err)
	}
	*d = DateOnly(parsed)
	return nil
}

// TimePtr returns the date as a *time.Time (midnight UTC), the form the domain
// model carries. Always non-nil.
func (d DateOnly) TimePtr() *time.Time {
	t := time.Time(d)
	return &t
}

// DateOnlyPtr converts an optional date-bearing time, preserving nil so an absent
// value serialises as null rather than a zero date.
func DateOnlyPtr(t *time.Time) *DateOnly {
	if t == nil {
		return nil
	}
	d := DateOnly(*t)
	return &d
}
