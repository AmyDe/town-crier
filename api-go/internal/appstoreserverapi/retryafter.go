package appstoreserverapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// parseRetryAfter parses an HTTP Retry-After header value, supporting both the
// delta-seconds form ("120") and the HTTP-date form
// ("Wed, 21 Oct 2015 07:28:00 GMT", parsed via the stdlib http.ParseTime).
// Malformed, negative, or absent values report ok=false so the caller falls
// back to a default wait; a past HTTP-date clamps to zero. now anchors the
// HTTP-date delta computation.
func parseRetryAfter(header string, now time.Time) (time.Duration, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0, false
	}

	if seconds, err := strconv.Atoi(header); err == nil {
		if seconds < 0 {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}

	if target, err := http.ParseTime(header); err == nil {
		delta := target.Sub(now)
		if delta < 0 {
			delta = 0
		}
		return delta, true
	}

	return 0, false
}
