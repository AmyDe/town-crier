package profiles

// zonePreferencesDocument is the stored JSON shape of a ZonePreferences entry
// in the users.zone_preferences column.
type zonePreferencesDocument struct {
	NewApplicationPush  bool `json:"newApplicationPush"`
	NewApplicationEmail bool `json:"newApplicationEmail"`
	DecisionPush        bool `json:"decisionPush"`
	DecisionEmail       bool `json:"decisionEmail"`
}

// coalesceTrue defaults an absent (nil) nullable flag to true, preserving the
// opt-in default for rows written before the flag existed.
func coalesceTrue(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}
