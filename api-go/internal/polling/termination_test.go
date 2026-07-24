package polling

import "testing"

// TestTerminationReason_TelemetryValue pins the span-tag string each
// TerminationReason maps to — these strings are stable (existing App
// Insights queries depend on them), so a new reason must be added here
// explicitly rather than falling through to the default.
func TestTerminationReason_TelemetryValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		reason TerminationReason
		want   string
	}{
		{name: "natural", reason: TerminationNatural, want: "Natural"},
		{name: "time-bounded", reason: TerminationTimeBounded, want: "TimeBounded"},
		{name: "rate-limited", reason: TerminationRateLimited, want: "RateLimited"},
		{name: "timeout", reason: TerminationTimeout, want: "Timeout"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.reason.TelemetryValue(); got != tc.want {
				t.Errorf("TelemetryValue(): got %q, want %q", got, tc.want)
			}
		})
	}
}
