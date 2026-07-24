package polling

// TerminationReason records how a poll cycle ended. It drives both the next-run
// cadence and the worker's exit code. The telemetry value strings are stable so
// existing App Insights queries keep working.
type TerminationReason int

const (
	// TerminationNatural means every active authority was processed before the
	// cycle ended.
	TerminationNatural TerminationReason = iota
	// TerminationTimeBounded means the cycle was cut short by the cycle budget or
	// the soft handler budget.
	TerminationTimeBounded
	// TerminationRateLimited means the cycle stopped early because PlanIt returned
	// HTTP 429.
	TerminationRateLimited
	// TerminationTimeout means the cycle stopped early because a PlanIt fetch
	// (page fetch or hydration) exceeded its client-side timeout — distinct
	// from TerminationNatural ("nothing happened") because a timeout is a real
	// signal that PlanIt needs space, even though it never surfaced as an
	// explicit 429.
	TerminationTimeout
)

// TelemetryValue returns the span-tag string for this reason.
func (r TerminationReason) TelemetryValue() string {
	switch r {
	case TerminationTimeBounded:
		return "TimeBounded"
	case TerminationRateLimited:
		return "RateLimited"
	case TerminationTimeout:
		return "Timeout"
	case TerminationNatural:
		return "Natural"
	default:
		return "Natural"
	}
}
