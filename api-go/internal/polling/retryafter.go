// Package polling is the Go port of the .NET TownCrier.Application.Polling and
// TownCrier.Infrastructure.Polling slices: the Service-Bus-triggered adaptive
// PlanIt poll (WORKER_MODE=poll-sb). It owns the trigger orchestrator, the
// PlanIt ingestion handler, the next-run scheduler, the Cosmos etag-CAS lease
// and poll-state stores, and the cycle-alternating authority providers.
//
// The crash-safety model is receive-and-delete + publish-after-consume per
// ADR 0024 (2026-04-22 amendment): the orchestrator acquires a Cosmos lease,
// destructively receives one trigger, runs the handler, publishes the next
// scheduled trigger, then releases the lease. There is no Service Bus
// Complete/Abandon — the safety-net bootstrap (internal/worker) is the sole
// recovery path when anything fails between receive and publish.
package polling

import (
	"strconv"
	"strings"
	"time"
)

// ParseRetryAfter parses an HTTP Retry-After header value, supporting both the
// delta-seconds form ("120") and the HTTP-date form
// ("Wed, 21 Oct 2015 07:28:00 GMT"). It mirrors .NET RetryAfterParser.Parse:
// malformed, negative, or absent values report ok=false so callers fall back to
// a default policy; a past HTTP-date clamps to zero. now anchors the HTTP-date
// delta computation.
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
