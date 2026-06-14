package polling

// TerminationReason records how a poll cycle ended. It mirrors .NET
// PollTerminationReason and drives both the next-run cadence and the worker's
// exit code. The telemetry value strings match .NET's
// PollTerminationReason.ToTelemetryValue() so existing App Insights queries keep
// working across the cutover.
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
)

// TelemetryValue returns the span-tag string for this reason, matching .NET's
// PollTerminationReason.ToTelemetryValue().
func (r TerminationReason) TelemetryValue() string {
	switch r {
	case TerminationTimeBounded:
		return "TimeBounded"
	case TerminationRateLimited:
		return "RateLimited"
	case TerminationNatural:
		return "Natural"
	default:
		return "Natural"
	}
}
