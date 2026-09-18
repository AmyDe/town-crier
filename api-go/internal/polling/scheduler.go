package polling

import "time"

// Jitter yields a symmetric random offset in [-bound, +bound]. It is injected so
// scheduler tests stay deterministic; production uses a math/rand/v2-backed
// implementation (operational de-synchronisation, not security-sensitive).
type Jitter interface {
	NextOffset(bound time.Duration) time.Duration
}

// SchedulerOptions are the tunables for NextRunScheduler.
type SchedulerOptions struct {
	NaturalCadence     time.Duration
	TimeBoundedCadence time.Duration
	RetryAfterCap      time.Duration
	RateLimitDefault   time.Duration
	TimeoutCadence     time.Duration
	JitterBound        time.Duration
}

// DefaultSchedulerOptions returns the production scheduler tunables.
func DefaultSchedulerOptions() SchedulerOptions {
	return SchedulerOptions{
		NaturalCadence:     1 * time.Hour,
		TimeBoundedCadence: 1 * time.Minute,
		// Equal to NaturalCadence: one trigger chain serves every lane, so a
		// longer hint would stall all ingest for longer than a quiet cycle.
		RetryAfterCap:    1 * time.Hour,
		RateLimitDefault: 5 * time.Minute,
		// Deliberately longer than NaturalCadence: a timeout backs off further
		// than a quiet cycle.
		TimeoutCadence: 2 * time.Hour,
		JitterBound:    10 * time.Second,
	}
}

// NextRunScheduler computes when the next poll trigger should be enqueued, given
// how the previous cycle ended.
type NextRunScheduler struct {
	opts   SchedulerOptions
	jitter Jitter
}

// NewNextRunScheduler wires the scheduler with its options and jitter source.
func NewNextRunScheduler(opts SchedulerOptions, jitter Jitter) *NextRunScheduler {
	return &NextRunScheduler{opts: opts, jitter: jitter}
}

// ComputeNextRun returns the absolute time the next trigger should enqueue.
// retryAfter is the optional Retry-After hint from a 429 (nil when absent). Only
// the rate-limited path consults retryAfter and applies jitter.
func (s *NextRunScheduler) ComputeNextRun(reason TerminationReason, retryAfter *time.Duration, now time.Time) time.Time {
	switch reason {
	case TerminationRateLimited:
		return now.Add(s.rateLimitedDelay(retryAfter))
	case TerminationTimeBounded:
		return now.Add(s.opts.TimeBoundedCadence)
	case TerminationTimeout:
		return now.Add(s.timeoutDelay())
	case TerminationNatural:
		return now.Add(s.opts.NaturalCadence)
	default:
		return now.Add(s.opts.NaturalCadence)
	}
}

// rateLimitedDelay caps the Retry-After hint at RetryAfterCap (falling back to
// RateLimitDefault when absent) and adds a symmetric jitter.
func (s *NextRunScheduler) rateLimitedDelay(retryAfter *time.Duration) time.Duration {
	base := s.opts.RateLimitDefault
	if retryAfter != nil {
		base = *retryAfter
		if base > s.opts.RetryAfterCap {
			base = s.opts.RetryAfterCap
		}
	}
	return base + s.jitter.NextOffset(s.opts.JitterBound)
}

// timeoutDelay adds a symmetric jitter to TimeoutCadence, mirroring
// rateLimitedDelay's base+jitter shape.
func (s *NextRunScheduler) timeoutDelay() time.Duration {
	return s.opts.TimeoutCadence + s.jitter.NextOffset(s.opts.JitterBound)
}
