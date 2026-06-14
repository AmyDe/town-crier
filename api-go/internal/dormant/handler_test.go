package dormant

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// --- hand-written fakes -----------------------------------------------------

type fakeFinder struct {
	dormant   []*profiles.UserProfile
	err       error
	gotCutoff time.Time
}

func (f *fakeFinder) Dormant(_ context.Context, cutoff time.Time) ([]*profiles.UserProfile, error) {
	f.gotCutoff = cutoff
	return f.dormant, f.err
}

// cascadeRecorder records, per user id, which cascade steps ran and in what
// order, so the test can assert the full erasure cascade fired for each account.
type cascadeRecorder struct {
	steps      map[string][]string
	notifErr   error
	zoneErr    error
	savedErr   error
	deviceErr  error
	stateErr   error
	profileErr map[string]error // keyed by user id
}

func newCascadeRecorder() *cascadeRecorder {
	return &cascadeRecorder{steps: map[string][]string{}, profileErr: map[string]error{}}
}

func (c *cascadeRecorder) record(userID, step string) {
	c.steps[userID] = append(c.steps[userID], step)
}

func (c *cascadeRecorder) DeleteAllNotifications(_ context.Context, userID string) error {
	c.record(userID, "notifications")
	return c.notifErr
}

func (c *cascadeRecorder) DeleteAllWatchZones(_ context.Context, userID string) error {
	c.record(userID, "watchzones")
	return c.zoneErr
}

func (c *cascadeRecorder) DeleteAllSavedApplications(_ context.Context, userID string) error {
	c.record(userID, "saved")
	return c.savedErr
}

func (c *cascadeRecorder) DeleteAllDeviceRegistrations(_ context.Context, userID string) error {
	c.record(userID, "devices")
	return c.deviceErr
}

func (c *cascadeRecorder) DeleteNotificationState(_ context.Context, userID string) error {
	c.record(userID, "state")
	return c.stateErr
}

func (c *cascadeRecorder) DeleteProfile(_ context.Context, userID string) error {
	c.record(userID, "profile")
	return c.profileErr[userID]
}

type fakeAuth0 struct {
	deleted   []string
	deleteErr error
}

func (a *fakeAuth0) DeleteUser(_ context.Context, userID string) error {
	if a.deleteErr != nil {
		return a.deleteErr
	}
	a.deleted = append(a.deleted, userID)
	return nil
}

func profile(t *testing.T, userID string) *profiles.UserProfile {
	t.Helper()
	p, err := profiles.NewProfile(userID, "", time.Now())
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	return p
}

func newHandler(finder Finder, cascade *cascadeRecorder, auth0 Auth0Deleter, now time.Time) *Handler {
	stores := Stores{
		Notifications:       cascade,
		WatchZones:          cascade,
		SavedApplications:   cascade,
		DeviceRegistrations: cascade,
		NotificationState:   cascade,
		Profiles:            cascade,
		Auth0:               auth0,
	}
	return New(finder, stores, slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return now })
}

// --- tests ------------------------------------------------------------------

func TestHandler_Run_CutoffIs12MonthsBeforeNow(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	finder := &fakeFinder{}
	h := newHandler(finder, newCascadeRecorder(), &fakeAuth0{}, now)

	if _, err := h.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := now.AddDate(0, -12, 0)
	if !finder.gotCutoff.Equal(want) {
		t.Errorf("cutoff: got %v, want %v (12 months before now)", finder.gotCutoff, want)
	}
}

func TestHandler_Run_DeletesEachDormantAccountWithFullCascade(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	finder := &fakeFinder{dormant: []*profiles.UserProfile{profile(t, "auth0|a"), profile(t, "auth0|b")}}
	cascade := newCascadeRecorder()
	auth0 := &fakeAuth0{}
	h := newHandler(finder, cascade, auth0, now)

	deleted, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if deleted != 2 {
		t.Errorf("deleted count: got %d, want 2", deleted)
	}

	wantSteps := []string{"notifications", "watchzones", "saved", "devices", "state", "profile"}
	for _, id := range []string{"auth0|a", "auth0|b"} {
		got := cascade.steps[id]
		if len(got) != len(wantSteps) {
			t.Fatalf("%s cascade steps: got %v, want %v", id, got, wantSteps)
		}
		for i := range wantSteps {
			if got[i] != wantSteps[i] {
				t.Errorf("%s cascade order at %d: got %q, want %q", id, i, got[i], wantSteps[i])
			}
		}
	}
	if len(auth0.deleted) != 2 || auth0.deleted[0] != "auth0|a" || auth0.deleted[1] != "auth0|b" {
		t.Errorf("auth0 deletes: got %v, want [auth0|a auth0|b]", auth0.deleted)
	}
}

func TestHandler_Run_NoDormantAccountsDeletesNothing(t *testing.T) {
	t.Parallel()
	finder := &fakeFinder{}
	cascade := newCascadeRecorder()
	auth0 := &fakeAuth0{}
	h := newHandler(finder, cascade, auth0, time.Now())

	deleted, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted count: got %d, want 0", deleted)
	}
	if len(auth0.deleted) != 0 {
		t.Errorf("auth0 deletes: got %v, want none", auth0.deleted)
	}
}

func TestHandler_Run_FinderErrorPropagates(t *testing.T) {
	t.Parallel()
	finder := &fakeFinder{err: errors.New("cosmos down")}
	h := newHandler(finder, newCascadeRecorder(), &fakeAuth0{}, time.Now())

	if _, err := h.Run(context.Background()); err == nil {
		t.Fatal("expected error when the dormant scan fails")
	}
}

func TestHandler_Run_ProfileNotFoundIsToleratedAndCounted(t *testing.T) {
	t.Parallel()
	// A concurrent delete between the scan and the cascade leaves the profile
	// gone; the cascade must tolerate ErrNotFound on the profile delete and still
	// count the account as removed (its end state is achieved), mirroring .NET's
	// UserProfileNotFoundException catch.
	finder := &fakeFinder{dormant: []*profiles.UserProfile{profile(t, "auth0|gone")}}
	cascade := newCascadeRecorder()
	cascade.profileErr["auth0|gone"] = profiles.ErrNotFound
	h := newHandler(finder, cascade, &fakeAuth0{}, time.Now())

	deleted, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted count: got %d, want 1 (not-found is tolerated)", deleted)
	}
}

func TestHandler_Run_ChildCascadeErrorAbortsThatAccountButContinues(t *testing.T) {
	t.Parallel()
	// A child-step failure leaves the profile intact so the next run retries; the
	// account is NOT counted as deleted, and the run continues to the next account.
	finder := &fakeFinder{dormant: []*profiles.UserProfile{profile(t, "auth0|fails"), profile(t, "auth0|ok")}}
	cascade := newCascadeRecorder()
	cascade.notifErr = errors.New("notifications delete failed")
	h := newHandler(finder, cascade, &fakeAuth0{}, time.Now())

	deleted, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Both accounts hit the notifications step first; both fail it, so none are
	// counted, but the run completes without erroring out.
	if deleted != 0 {
		t.Errorf("deleted count: got %d, want 0 (child failure aborts that account)", deleted)
	}
}
