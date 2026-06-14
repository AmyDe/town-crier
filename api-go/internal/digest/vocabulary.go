package digest

import "strings"

// ukDisplayString maps a raw PlanIt app_state ("Permitted", "Conditions",
// "Rejected", "Appealed") to the UK planning term residents recognise
// ("Approved", "Approved with conditions", "Refused", "Refusal appealed"),
// returning "" for a nil or unrecognised input. Matching is case-insensitive to
// tolerate upstream casing drift. Port of .NET UkPlanningVocabulary.GetDisplayString.
func ukDisplayString(planItAppState *string) string {
	if planItAppState == nil {
		return ""
	}
	state := strings.TrimSpace(*planItAppState)
	switch {
	case strings.EqualFold(state, "Permitted"):
		return "Approved"
	case strings.EqualFold(state, "Conditions"):
		return "Approved with conditions"
	case strings.EqualFold(state, "Rejected"):
		return "Refused"
	case strings.EqualFold(state, "Appealed"):
		return "Refusal appealed"
	default:
		return ""
	}
}
