package erasure

import (
	"context"
	"errors"
	"slices"
	"testing"
)

// --- hand-written fakes -----------------------------------------------------

// recordingChild appends "<label>:<userID>" to a shared log on each call and
// returns its configured error, so a test can assert both coverage and order
// across the whole cascade.
type recordingChild struct {
	label string
	log   *[]string
	err   error
}

func (c recordingChild) DeleteAllByUserID(_ context.Context, userID string) error {
	*c.log = append(*c.log, c.label+":"+userID)
	return c.err
}

// recordingAnonymiser records "offerCodes:<userID>" and returns its configured
// error, standing in for the offer-code redemption anonymiser.
type recordingAnonymiser struct {
	log *[]string
	err error
}

func (a recordingAnonymiser) AnonymiseRedemptionsByUserID(_ context.Context, userID string) error {
	*a.log = append(*a.log, "offerCodes:"+userID)
	return a.err
}

// recordingProfile records "profile:<userID>" and returns its configured error.
type recordingProfile struct {
	log *[]string
	err error
}

func (p recordingProfile) Delete(_ context.Context, userID string) error {
	*p.log = append(*p.log, "profile:"+userID)
	return p.err
}

// recordingAuth0 records "auth0:<userID>" and returns its configured error.
type recordingAuth0 struct {
	log *[]string
	err error
}

func (a recordingAuth0) DeleteUser(_ context.Context, userID string) error {
	*a.log = append(*a.log, "auth0:"+userID)
	return a.err
}

// recordingDeleters wires a Deleters whose every step appends to the shared log,
// so a single recorder captures the entire cascade order.
func recordingDeleters(log *[]string) Deleters {
	return Deleters{
		Notifications:       recordingChild{label: "notifications", log: log},
		WatchZones:          recordingChild{label: "watchZones", log: log},
		SavedApplications:   recordingChild{label: "savedApplications", log: log},
		DeviceRegistrations: recordingChild{label: "deviceRegistrations", log: log},
		NotificationState:   recordingChild{label: "notificationState", log: log},
		OfferCodes:          recordingAnonymiser{log: log},
		Profile:             recordingProfile{log: log},
		Auth0:               recordingAuth0{log: log},
	}
}

// --- tests ------------------------------------------------------------------

func TestCascade_RunsEveryStepInFixedOrder(t *testing.T) {
	t.Parallel()
	var log []string
	d := recordingDeleters(&log)

	if err := Cascade(context.Background(), "auth0|abc", d); err != nil {
		t.Fatalf("Cascade: %v", err)
	}

	want := []string{
		"notifications:auth0|abc",
		"watchZones:auth0|abc",
		"savedApplications:auth0|abc",
		"deviceRegistrations:auth0|abc",
		"notificationState:auth0|abc",
		"offerCodes:auth0|abc",
		"profile:auth0|abc",
		"auth0:auth0|abc",
	}
	if !slices.Equal(log, want) {
		t.Errorf("cascade order: got %v, want %v", log, want)
	}
}

func TestCascade_ChildFailureAbortsBeforeProfileAndAuth0(t *testing.T) {
	t.Parallel()
	var log []string
	d := recordingDeleters(&log)
	d.WatchZones = recordingChild{label: "watchZones", log: &log, err: errors.New("cosmos down")}

	err := Cascade(context.Background(), "auth0|abc", d)
	if err == nil {
		t.Fatal("expected error when a child step fails")
	}
	if slices.Contains(log, "profile:auth0|abc") {
		t.Errorf("profile must not run after a child failure: %v", log)
	}
	if slices.Contains(log, "auth0:auth0|abc") {
		t.Errorf("auth0 must not run after a child failure: %v", log)
	}
}

func TestCascade_OfferCodeAnonymiseFailureAbortsBeforeProfileAndAuth0(t *testing.T) {
	t.Parallel()
	var log []string
	d := recordingDeleters(&log)
	d.OfferCodes = recordingAnonymiser{log: &log, err: errors.New("cosmos down")}

	err := Cascade(context.Background(), "auth0|abc", d)
	if err == nil {
		t.Fatal("expected error when offer-code anonymisation fails")
	}
	if slices.Contains(log, "profile:auth0|abc") {
		t.Errorf("profile must not run after an offer-code failure: %v", log)
	}
	if slices.Contains(log, "auth0:auth0|abc") {
		t.Errorf("auth0 must not run after an offer-code failure: %v", log)
	}
}

func TestCascade_Auth0RunsLastAfterEveryOtherStep(t *testing.T) {
	t.Parallel()
	var log []string
	d := recordingDeleters(&log)
	d.Auth0 = recordingAuth0{log: &log, err: errors.New("auth0 m2m down")}

	err := Cascade(context.Background(), "auth0|abc", d)
	if err == nil {
		t.Fatal("expected error when the auth0 step fails")
	}
	// All five children plus the profile ran before the (failed) auth0 attempt,
	// and the auth0 attempt was recorded last.
	want := []string{
		"notifications:auth0|abc",
		"watchZones:auth0|abc",
		"savedApplications:auth0|abc",
		"deviceRegistrations:auth0|abc",
		"notificationState:auth0|abc",
		"offerCodes:auth0|abc",
		"profile:auth0|abc",
		"auth0:auth0|abc",
	}
	if !slices.Equal(log, want) {
		t.Errorf("auth0-last order: got %v, want %v", log, want)
	}
	profileIdx := slices.Index(log, "profile:auth0|abc")
	auth0Idx := slices.Index(log, "auth0:auth0|abc")
	if profileIdx < 0 || auth0Idx < 0 || profileIdx >= auth0Idx {
		t.Errorf("profile must run before auth0: profileIdx=%d auth0Idx=%d", profileIdx, auth0Idx)
	}
}

func TestCascade_ProfileAbsentToleratedProceedsToAuth0(t *testing.T) {
	t.Parallel()
	var log []string
	gone := errors.New("profile already gone")
	d := recordingDeleters(&log)
	d.Profile = recordingProfile{log: &log, err: gone}
	d.ProfileAbsent = func(e error) bool { return errors.Is(e, gone) }

	if err := Cascade(context.Background(), "auth0|abc", d); err != nil {
		t.Fatalf("Cascade should tolerate an absent profile: %v", err)
	}
	if !slices.Contains(log, "auth0:auth0|abc") {
		t.Errorf("auth0 must run after a tolerated profile-absent: %v", log)
	}
}

func TestCascade_ProfileErrorNotAbsentAbortsBeforeAuth0(t *testing.T) {
	t.Parallel()

	t.Run("predicate returns false", func(t *testing.T) {
		t.Parallel()
		var log []string
		fatal := errors.New("profile delete failed")
		d := recordingDeleters(&log)
		d.Profile = recordingProfile{log: &log, err: fatal}
		d.ProfileAbsent = func(error) bool { return false }

		err := Cascade(context.Background(), "auth0|abc", d)
		if !errors.Is(err, fatal) {
			t.Fatalf("expected the profile error to propagate, got %v", err)
		}
		if slices.Contains(log, "auth0:auth0|abc") {
			t.Errorf("auth0 must not run after a fatal profile error: %v", log)
		}
	})

	t.Run("nil predicate treats every profile error as fatal", func(t *testing.T) {
		t.Parallel()
		var log []string
		fatal := errors.New("profile delete failed")
		d := recordingDeleters(&log)
		d.Profile = recordingProfile{log: &log, err: fatal}
		d.ProfileAbsent = nil

		err := Cascade(context.Background(), "auth0|abc", d)
		if !errors.Is(err, fatal) {
			t.Fatalf("expected the profile error to propagate with a nil predicate, got %v", err)
		}
		if slices.Contains(log, "auth0:auth0|abc") {
			t.Errorf("auth0 must not run after a fatal profile error: %v", log)
		}
	})
}

// fakeByUserIDDeleter records that DeleteByUserID was called with the user id,
// standing in for the notification-state store whose method is DeleteByUserID.
type fakeByUserIDDeleter struct {
	called []string
}

func (f *fakeByUserIDDeleter) DeleteByUserID(_ context.Context, userID string) error {
	f.called = append(f.called, userID)
	return nil
}

func TestNotificationStateChild_DelegatesToDeleteByUserID(t *testing.T) {
	t.Parallel()
	fake := &fakeByUserIDDeleter{}
	child := NotificationStateChild(fake)

	if err := child.DeleteAllByUserID(context.Background(), "auth0|abc"); err != nil {
		t.Fatalf("DeleteAllByUserID: %v", err)
	}
	if !slices.Equal(fake.called, []string{"auth0|abc"}) {
		t.Errorf("expected DeleteByUserID called with auth0|abc, got %v", fake.called)
	}
}
