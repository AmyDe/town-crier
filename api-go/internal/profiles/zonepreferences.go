package profiles

// DefaultZonePreferences returns the default per-zone preferences:
// every notification channel opted in.
func DefaultZonePreferences() ZonePreferences {
	return ZonePreferences{
		NewApplicationPush:  true,
		NewApplicationEmail: true,
		DecisionPush:        true,
		DecisionEmail:       true,
	}
}

// GetZonePreferences returns the stored per-zone notification preferences for
// zoneID, or the all-on defaults when the user has never customised that zone.
func (p *UserProfile) GetZonePreferences(zoneID string) ZonePreferences {
	if prefs, ok := p.ZonePreferences[zoneID]; ok {
		return prefs
	}
	return DefaultZonePreferences()
}

// SetZonePreferences stores (replacing any existing) the per-zone preferences
// for zoneID. It is safe to call on a profile whose preference map was never
// initialised.
func (p *UserProfile) SetZonePreferences(zoneID string, prefs ZonePreferences) {
	if p.ZonePreferences == nil {
		p.ZonePreferences = map[string]ZonePreferences{}
	}
	p.ZonePreferences[zoneID] = prefs
}
