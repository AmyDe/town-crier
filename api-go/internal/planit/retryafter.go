package planit

import (
	"strconv"
	"strings"
	"time"
)

// ParseRetryAfter parses an HTTP Retry-After header value, supporting both the
// delta-seconds form ("120") and the HTTP-date form
// ("Wed, 21 Oct 2015 07:28:00 GMT"). Malformed, negative, or absent values
// report ok=false so callers fall back to a default policy; a past HTTP-date
// clamps to zero. now anchors the HTTP-date delta computation.
func ParseRetryAfter(header string, now time.Time) (time.Duration, bool) {
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

	if target, err := http1123(header); err == nil {
		delta := target.Sub(now)
		if delta < 0 {
			return 0, true
		}
		return delta, true
	}

	return 0, false
}

// http1123 parses an HTTP-date in the three formats RFC 7231 permits, in the
// order browsers and the Go net/http stack try them. The result is in UTC so
// the delta against an absolute now is computed in a single timezone.
func http1123(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC1123, time.RFC1123Z, time.RFC850, time.ANSIC} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}
	// time.RFC1123 expects "GMT"; some servers send "UTC" or an offset. Fall
	// back to RFC3339 for robustness, then give up.
	t, err := time.Parse(time.RFC3339, value)
	return t.UTC(), err
}
