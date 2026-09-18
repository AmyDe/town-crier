package polling

import (
	"testing"
	"time"
)

func TestNextRunScheduler_ComputeNextRun(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	opts := DefaultSchedulerOptions()

	tests := []struct {
		name       string
		reason     TerminationReason
		retryAfter time.Duration
		hasRetry   bool
		want       time.Time
	}{
		{
			name:   "natural uses natural cadence",
			reason: TerminationNatural,
			want:   now.Add(1 * time.Hour),
		},
		{
			name:   "time-bounded uses short resume cadence",
			reason: TerminationTimeBounded,
			want:   now.Add(1 * time.Minute),
		},
		{
			name:       "rate-limited honours retry-after",
			reason:     TerminationRateLimited,
			retryAfter: 90 * time.Second,
			hasRetry:   true,
			want:       now.Add(90 * time.Second),
		},
		{
			name:       "rate-limited honours a hint below the cap",
			reason:     TerminationRateLimited,
			retryAfter: 50 * time.Minute,
			hasRetry:   true,
			want:       now.Add(50 * time.Minute),
		},
		{
			name:       "rate-limited hint exactly at the cap is honoured",
			reason:     TerminationRateLimited,
			retryAfter: 1 * time.Hour,
			hasRetry:   true,
			want:       now.Add(1 * time.Hour),
		},
		{
			name:       "rate-limited caps an oversized retry-after",
			reason:     TerminationRateLimited,
			retryAfter: 3 * time.Hour,
			hasRetry:   true,
			want:       now.Add(1 * time.Hour),
		},
		{
			name:   "rate-limited without retry-after uses default",
			reason: TerminationRateLimited,
			want:   now.Add(5 * time.Minute),
		},
		{
			name:   "timeout uses timeout cadence",
			reason: TerminationTimeout,
			want:   now.Add(2 * time.Hour),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := NewNextRunScheduler(opts, zeroJitter{})
			var retry *time.Duration
			if tc.hasRetry {
				ra := tc.retryAfter
				retry = &ra
			}
			got := s.ComputeNextRun(tc.reason, retry, now)
			if !got.Equal(tc.want) {
				t.Errorf("ComputeNextRun(%v): got %v, want %v", tc.reason, got, tc.want)
			}
		})
	}
}

func TestDefaultSchedulerOptions_RetryAfterCap(t *testing.T) {
	t.Parallel()
	got := DefaultSchedulerOptions().RetryAfterCap
	want := 1 * time.Hour
	if got != want {
		t.Errorf("RetryAfterCap = %v, want %v", got, want)
	}
}

func TestNextRunScheduler_RateLimitedCapAppliesJitterAfterCapping(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	opts := DefaultSchedulerOptions()
	s := NewNextRunScheduler(opts, fixedJitter{offset: opts.JitterBound})

	ra := 3 * time.Hour
	got := s.ComputeNextRun(TerminationRateLimited, &ra, now)
	// A literal, not opts.RetryAfterCap, so a change to the cap fails this test.
	want := now.Add(1*time.Hour + opts.JitterBound)
	if !got.Equal(want) {
		t.Errorf("capped+jittered next run: got %v, want %v", got, want)
	}
}

func TestNextRunScheduler_RateLimitedAppliesJitter(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	opts := DefaultSchedulerOptions()
	// A fixed +7s jitter must be added on top of the (capped) base delay only on
	// the rate-limited path.
	s := NewNextRunScheduler(opts, fixedJitter{offset: 7 * time.Second})

	ra := 90 * time.Second
	got := s.ComputeNextRun(TerminationRateLimited, &ra, now)
	want := now.Add(90*time.Second + 7*time.Second)
	if !got.Equal(want) {
		t.Errorf("rate-limited jittered next run: got %v, want %v", got, want)
	}

	// Natural cadence does NOT apply jitter (only the rate-limited branch does).
	gotNatural := s.ComputeNextRun(TerminationNatural, nil, now)
	if !gotNatural.Equal(now.Add(1 * time.Hour)) {
		t.Errorf("natural cadence should not jitter: got %v, want %v", gotNatural, now.Add(1*time.Hour))
	}
}

func TestNextRunScheduler_TimeoutAppliesJitter(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	opts := DefaultSchedulerOptions()
	// A fixed +7s jitter must be added on top of the timeout cadence, mirroring
	// the rate-limited path's jitter application.
	s := NewNextRunScheduler(opts, fixedJitter{offset: 7 * time.Second})

	got := s.ComputeNextRun(TerminationTimeout, nil, now)
	want := now.Add(2*time.Hour + 7*time.Second)
	if !got.Equal(want) {
		t.Errorf("timeout jittered next run: got %v, want %v", got, want)
	}
}

// zeroJitter and fixedJitter are hand-written jitter doubles for deterministic
// scheduler tests.
type zeroJitter struct{}

func (zeroJitter) NextOffset(time.Duration) time.Duration { return 0 }

type fixedJitter struct{ offset time.Duration }

func (f fixedJitter) NextOffset(time.Duration) time.Duration { return f.offset }
