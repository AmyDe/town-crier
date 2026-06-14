package polling

import "time"

// Jitter yields a symmetric random offset in [-bound, +bound]. It is injected so
// scheduler tests stay deterministic; production uses a math/rand/v2-backed
// implementation (operational de-synchronisation, not security-sensitive).
type Jitter interface {
	NextOffset(bound time.Duration) time.Duration
}

// SchedulerOptions are the next-run scheduler tunables. Defaults mirror .NET
// PollNextRunSchedulerOptions: 5m natural cadence, 1m resume after a time-bounded
// cut-off, 3h cap on a Retry-After hint, 5m rate-limit default, 10s jitter bound.
type SchedulerOptions struct {
	NaturalCadence     time.Duration
	TimeBoundedCadence time.Duration
	RetryAfterCap      time.Duration
	RateLimitDefault   time.Duration
	JitterBound        time.Duration
}

// DefaultSchedulerOptions returns the .NET-default tunables.
func DefaultSchedulerOptions() SchedulerOptions {
	return SchedulerOptions{
		NaturalCadence:     5 * time.Minute,
		TimeBoundedCadence: 1 * time.Minute,
		RetryAfterCap:      3 * time.Hour,
		RateLimitDefault:   5 * time.Minute,
		JitterBound:        10 * time.Second,
	}
}

// NextRunScheduler computes when the next poll trigger should be enqueued, given
// how the previous cycle ended. It is the Go port of .NET PollNextRunScheduler.
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
// the rate-limited path consults retryAfter and applies jitter, matching .NET.
func (s *NextRunScheduler) ComputeNextRun(reason TerminationReason, retryAfter *time.Duration, now time.Time) time.Time {
	switch reason {
	case TerminationRateLimited:
		return now.Add(s.rateLimitedDelay(retryAfter))
	case TerminationTimeBounded:
		return now.Add(s.opts.TimeBoundedCadence)
	case TerminationNatural:
		return now.Add(s.opts.NaturalCadence)
	default:
		return now.Add(s.opts.NaturalCadence)
	}
}

// rateLimitedDelay caps the Retry-After hint at RetryAfterCap (falling back to
// RateLimitDefault when absent) and adds a symmetric jitter, mirroring .NET
// ComputeRateLimitedDelay.
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
