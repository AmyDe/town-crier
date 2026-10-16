package subscriptionevents

// EventType names a kind of subscription change.
type EventType string

// EventType values as they appear on the wire.
const (
	EventTrialStarted   EventType = "trial_started"
	EventTrialConverted EventType = "trial_converted"
	EventPaidStarted    EventType = "paid_started"
	EventUpgraded       EventType = "upgraded"
	EventDowngraded     EventType = "downgraded"
	EventPlanChanged    EventType = "plan_changed"
	EventRevertedToFree EventType = "reverted_to_free"
)

// Source names the code path that made a change.
type Source string

// Source values as they appear on the wire.
const (
	SourceAppStore  Source = "app_store"
	SourceOfferCode Source = "offer_code"
	SourceAdmin     Source = "admin"
	SourceSweep     Source = "sweep"
)

// Classify turns a before/after snapshot pair into an event. ok is false when
// the change is not worth reporting (no difference, a renewal, a grace entry).
func Classify(before, after Snapshot) (EventType, bool) {
	switch {
	case before == after:
		return "", false
	case after.Tier == 0 && before.Tier != 0:
		return EventRevertedToFree, true
	case before.Tier == 0 && after.Tier != 0:
		if after.InTrial {
			return EventTrialStarted, true
		}
		return EventPaidStarted, true
	case before.Tier == 0:
		return "", false
	case after.Tier > before.Tier:
		return EventUpgraded, true
	case after.Tier < before.Tier:
		return EventDowngraded, true
	case before.InTrial && !after.InTrial:
		return EventTrialConverted, true
	case before.Plan != after.Plan:
		return EventPlanChanged, true
	default:
		return "", false
	}
}
