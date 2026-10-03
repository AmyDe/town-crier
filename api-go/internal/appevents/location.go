package appevents

import (
	"time"
	_ "time/tzdata" // the runtime image has no tz database
)

// londonLocation cannot fail with the embedded tz database; UTC is a defensive fallback.
func londonLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		return time.UTC
	}
	return loc
}
