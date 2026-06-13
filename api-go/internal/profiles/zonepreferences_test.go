package profiles

import (
	"testing"
	"time"
)

func TestDefaultZonePreferences_AllChannelsOn(t *testing.T) {
	t.Parallel()
	got := DefaultZonePreferences()
	if !got.NewApplicationPush || !got.NewApplicationEmail || !got.DecisionPush || !got.DecisionEmail {
		t.Errorf("default zone preferences must be all-on, got %+v", got)
	}
}

func TestUserProfile_GetZonePreferences_DefaultsWhenAbsent(t *testing.T) {
	t.Parallel()
	p, _ := NewProfile("auth0|u", "", time.Now())
	got := p.GetZonePreferences("never-set")
	if got != DefaultZonePreferences() {
		t.Errorf("absent zone must return defaults, got %+v", got)
	}
}

func TestUserProfile_SetThenGetZonePreferences(t *testing.T) {
	t.Parallel()
	p, _ := NewProfile("auth0|u", "", time.Now())
	want := ZonePreferences{NewApplicationPush: false, NewApplicationEmail: true, DecisionPush: false, DecisionEmail: true}
	p.SetZonePreferences("zone-1", want)

	if got := p.GetZonePreferences("zone-1"); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	// Other zones still default.
	if got := p.GetZonePreferences("zone-2"); got != DefaultZonePreferences() {
		t.Errorf("unrelated zone changed: %+v", got)
	}
}

func TestUserProfile_SetZonePreferences_NilMapSafe(t *testing.T) {
	t.Parallel()
	p := &UserProfile{UserID: "u"} // ZonePreferences nil
	p.SetZonePreferences("z", DefaultZonePreferences())
	if got := p.GetZonePreferences("z"); got != DefaultZonePreferences() {
		t.Errorf("set on nil map failed: %+v", got)
	}
}
