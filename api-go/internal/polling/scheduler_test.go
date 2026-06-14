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
			name: "natural uses natural cadence",
			// 5m natural cadence, no jitter (jitter is zero in this test).
			reason: TerminationNatural,
			want:   now.Add(5 * time.Minute),
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
			name:       "rate-limited caps an oversized retry-after",
			reason:     TerminationRateLimited,
			retryAfter: 10 * time.Hour, // > 3h cap
			hasRetry:   true,
			want:       now.Add(3 * time.Hour),
		},
		{
			name:   "rate-limited without retry-after uses default",
			reason: TerminationRateLimited,
			// no retry-after -> RateLimitDefault (5m)
			want: now.Add(5 * time.Minute),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Zero jitter makes the result deterministic; jitter coverage is a
			// separate test below.
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

func TestNextRunScheduler_RateLimitedAppliesJitter(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	opts := DefaultSchedulerOptions()
	// A fixed +7s jitter must be added on top of the (capped) base delay only on
	// the rate-limited path, matching .NET's ComputeRateLimitedDelay.
	s := NewNextRunScheduler(opts, fixedJitter{offset: 7 * time.Second})

	ra := 90 * time.Second
	got := s.ComputeNextRun(TerminationRateLimited, &ra, now)
	want := now.Add(90*time.Second + 7*time.Second)
	if !got.Equal(want) {
		t.Errorf("rate-limited jittered next run: got %v, want %v", got, want)
	}

	// Natural cadence does NOT apply jitter (only the rate-limited branch does in
	// .NET PollNextRunScheduler).
	gotNatural := s.ComputeNextRun(TerminationNatural, nil, now)
	if !gotNatural.Equal(now.Add(5 * time.Minute)) {
		t.Errorf("natural cadence should not jitter: got %v, want %v", gotNatural, now.Add(5*time.Minute))
	}
}

// zeroJitter and fixedJitter are hand-written jitter doubles for deterministic
// scheduler tests.
type zeroJitter struct{}

func (zeroJitter) NextOffset(time.Duration) time.Duration { return 0 }

type fixedJitter struct{ offset time.Duration }

func (f fixedJitter) NextOffset(time.Duration) time.Duration { return f.offset }
