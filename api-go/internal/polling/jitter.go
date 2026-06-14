package polling

import (
	mathrand "math/rand/v2"
	"time"
)

// RandomJitter is the production Jitter: a symmetric offset uniformly drawn from
// [-bound, +bound]. The jitter de-synchronises re-enqueues across cycles, so it
// is operational rather than security-sensitive — math/rand/v2 is the correct
// tool (gosec G404 is a false positive here), matching internal/worker bootstrap.
type RandomJitter struct{}

// NewRandomJitter returns the production jitter source.
func NewRandomJitter() RandomJitter { return RandomJitter{} }

// NextOffset returns a value in [-bound, +bound]. A non-positive bound yields 0.
func (RandomJitter) NextOffset(bound time.Duration) time.Duration {
	if bound <= 0 {
		return 0
	}
	// Int64N(2*bound+1) yields [0, 2*bound]; subtracting bound centres it on zero.
	return time.Duration(mathrand.Int64N(int64(2*bound)+1)) - bound //nolint:gosec // non-security operational jitter
}
