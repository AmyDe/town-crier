package dormant

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/erasure"
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

// cascadeRecorder satisfies every erasure interface a child + profile delete
// needs: erasure.ChildDeleter (DeleteAllByUserID) is wired to all five child
// slots, erasure.RedemptionAnonymiser (AnonymiseRedemptionsByUserID) handles the
// offer-code scrub, erasure.ProfileDeleter (Delete) handles the profile step, and
// it counts how many cascade steps ran per user. Since all five children share
// DeleteAllByUserID, the test asserts the count of child invocations per account
// rather than distinct labels — the per-container ORDER assertion now lives in
// erasure's own test.
type cascadeRecorder struct {
	childCalls map[string]int   // child DeleteAllByUserID calls per user id
	offerCodes map[string]int   // AnonymiseRedemptionsByUserID calls per user id
	profile    map[string]bool  // profile Delete called per user id
	notifErr   error            // injected on the first (notifications) child call
	profileErr map[string]error // keyed by user id, returned from Delete
}

func newCascadeRecorder() *cascadeRecorder {
	return &cascadeRecorder{
		childCalls: map[string]int{},
		offerCodes: map[string]int{},
		profile:    map[string]bool{},
		profileErr: map[string]error{},
	}
}

func (c *cascadeRecorder) DeleteAllByUserID(_ context.Context, userID string) error {
	// The first child call per user is the notifications step; injecting notifErr
	// here models a child-container failure aborting the cascade before later
	// children, the profile, and Auth0 run.
	first := c.childCalls[userID] == 0
	c.childCalls[userID]++
	if first && c.notifErr != nil {
		return c.notifErr
	}
	return nil
}

func (c *cascadeRecorder) AnonymiseRedemptionsByUserID(_ context.Context, userID string) error {
	c.offerCodes[userID]++
	return nil
}

func (c *cascadeRecorder) Delete(_ context.Context, userID string) error {
	c.profile[userID] = true
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

func newHandler(finder Finder, cascade *cascadeRecorder, auth0 erasure.Auth0Deleter, now time.Time) *Handler {
	deleters := erasure.Deleters{
		Notifications:       cascade,
		WatchZones:          cascade,
		SavedApplications:   cascade,
		DeviceRegistrations: cascade,
		NotificationState:   cascade,
		OfferCodes:          cascade,
		Profile:             cascade,
		Auth0:               auth0,
		ProfileAbsent:       func(e error) bool { return errors.Is(e, profiles.ErrNotFound) },
	}
	return New(finder, deleters, slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)), func() time.Time { return now })
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

	// Each account runs all five child-container erasures, the offer-code
	// redemption anonymise, plus the profile delete.
	for _, id := range []string{"auth0|a", "auth0|b"} {
		if cascade.childCalls[id] != 5 {
			t.Errorf("%s child erasures: got %d, want 5", id, cascade.childCalls[id])
		}
		if cascade.offerCodes[id] != 1 {
			t.Errorf("%s offer-code anonymise: got %d, want 1", id, cascade.offerCodes[id])
		}
		if !cascade.profile[id] {
			t.Errorf("%s profile not deleted", id)
		}
	}
	// Auth0 runs last per account, after every Cosmos erasure.
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
	// gone; the cascade must tolerate ErrNotFound on the profile delete (via the
	// ProfileAbsent predicate) and still count the account as removed (its end
	// state is achieved).
	finder := &fakeFinder{dormant: []*profiles.UserProfile{profile(t, "auth0|gone")}}
	cascade := newCascadeRecorder()
	cascade.profileErr["auth0|gone"] = profiles.ErrNotFound
	auth0 := &fakeAuth0{}
	h := newHandler(finder, cascade, auth0, time.Now())

	deleted, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted count: got %d, want 1 (not-found is tolerated)", deleted)
	}
	// A tolerated profile-absent still proceeds to delete the Auth0 user.
	if len(auth0.deleted) != 1 || auth0.deleted[0] != "auth0|gone" {
		t.Errorf("auth0 deletes: got %v, want [auth0|gone]", auth0.deleted)
	}
}

func TestHandler_Run_ChildCascadeErrorAbortsThatAccountButContinues(t *testing.T) {
	t.Parallel()
	// A child-step failure leaves the profile intact so the next run retries; the
	// account is NOT counted as deleted, and the run continues to the next account.
	finder := &fakeFinder{dormant: []*profiles.UserProfile{profile(t, "auth0|fails"), profile(t, "auth0|ok")}}
	cascade := newCascadeRecorder()
	cascade.notifErr = errors.New("notifications delete failed")
	auth0 := &fakeAuth0{}
	h := newHandler(finder, cascade, auth0, time.Now())

	deleted, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Both accounts hit the notifications step first; both fail it, so none are
	// counted, but the run completes without erroring out.
	if deleted != 0 {
		t.Errorf("deleted count: got %d, want 0 (child failure aborts that account)", deleted)
	}
	// The profile and Auth0 must be untouched when a child step aborts the cascade.
	if cascade.profile["auth0|fails"] || cascade.profile["auth0|ok"] {
		t.Errorf("profile must survive a child-cascade failure")
	}
	if len(auth0.deleted) != 0 {
		t.Errorf("auth0 must not run after a child-cascade failure, got %v", auth0.deleted)
	}
}
