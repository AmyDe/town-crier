package subscriptionsweep

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// --- hand-written fakes -----------------------------------------------------

type fakeFinder struct {
	lapsed []*profiles.UserProfile
	err    error
	gotNow time.Time
}

func (f *fakeFinder) LapsedPaid(_ context.Context, now time.Time) ([]*profiles.UserProfile, error) {
	f.gotNow = now
	return f.lapsed, f.err
}

// fakeSaver records the profiles upserted back to Cosmos and can be primed with a
// per-user save error, so the test can isolate a single profile's failure.
type fakeSaver struct {
	saved   map[string]profiles.SubscriptionTier // tier at the moment of Save
	saveErr map[string]error                     // keyed by user id
}

func newFakeSaver() *fakeSaver {
	return &fakeSaver{saved: map[string]profiles.SubscriptionTier{}, saveErr: map[string]error{}}
}

func (s *fakeSaver) Save(_ context.Context, p *profiles.UserProfile) error {
	if err := s.saveErr[p.UserID]; err != nil {
		return err
	}
	s.saved[p.UserID] = p.Tier
	return nil
}

// fakeAuth0 records the tier synced to Auth0 per user and can be primed with a
// per-user error.
type fakeAuth0 struct {
	synced  map[string]string // user id -> tier string
	syncErr map[string]error
}

func newFakeAuth0() *fakeAuth0 {
	return &fakeAuth0{synced: map[string]string{}, syncErr: map[string]error{}}
}

func (a *fakeAuth0) UpdateSubscriptionTier(_ context.Context, userID, tier string) error {
	if err := a.syncErr[userID]; err != nil {
		return err
	}
	a.synced[userID] = tier
	return nil
}

func lapsedProfile(t *testing.T, userID string) *profiles.UserProfile {
	t.Helper()
	p, err := profiles.NewProfile(userID, "", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewProfile: %v", err)
	}
	// A paid grant that has already lapsed; the finder is responsible for the
	// EffectiveTier filtering, so the handler just downgrades whatever it returns.
	p.ActivateSubscription(profiles.TierPro, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	return p
}

func newHandler(finder Finder, saver Saver, auth0 Auth0Syncer, now time.Time) *Handler {
	return New(finder, saver, auth0, slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)), func() time.Time { return now })
}

// --- tests ------------------------------------------------------------------

func TestHandler_Run_PassesNowToFinder(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 23, 9, 0, 0, 0, time.UTC)
	finder := &fakeFinder{}
	h := newHandler(finder, newFakeSaver(), newFakeAuth0(), now)

	if _, err := h.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !finder.gotNow.Equal(now) {
		t.Errorf("now passed to finder: got %v, want %v", finder.gotNow, now)
	}
}

func TestHandler_Run_DowngradesEachLapsedProfileAndSyncsAuth0(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 6, 23, 9, 0, 0, 0, time.UTC)
	finder := &fakeFinder{lapsed: []*profiles.UserProfile{lapsedProfile(t, "auth0|a"), lapsedProfile(t, "auth0|b")}}
	saver := newFakeSaver()
	auth0 := newFakeAuth0()
	h := newHandler(finder, saver, auth0, now)

	downgraded, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if downgraded != 2 {
		t.Errorf("downgraded count: got %d, want 2", downgraded)
	}

	for _, id := range []string{"auth0|a", "auth0|b"} {
		// Each lapsed profile is reverted to Free in Cosmos before being saved.
		if tier, ok := saver.saved[id]; !ok || tier != profiles.TierFree {
			t.Errorf("%s saved tier: got %v (saved=%v), want Free", id, tier, ok)
		}
		// And Auth0 metadata is synced to the canonical "Free" string.
		if auth0.synced[id] != "Free" {
			t.Errorf("%s auth0 tier: got %q, want Free", id, auth0.synced[id])
		}
	}
}

func TestHandler_Run_NoLapsedProfilesDoesNothing(t *testing.T) {
	t.Parallel()
	saver := newFakeSaver()
	auth0 := newFakeAuth0()
	h := newHandler(&fakeFinder{}, saver, auth0, time.Now())

	downgraded, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if downgraded != 0 {
		t.Errorf("downgraded count: got %d, want 0", downgraded)
	}
	if len(saver.saved) != 0 || len(auth0.synced) != 0 {
		t.Errorf("nothing should be saved or synced: saved=%v synced=%v", saver.saved, auth0.synced)
	}
}

func TestHandler_Run_FinderErrorPropagates(t *testing.T) {
	t.Parallel()
	finder := &fakeFinder{err: errors.New("cosmos down")}
	h := newHandler(finder, newFakeSaver(), newFakeAuth0(), time.Now())

	if _, err := h.Run(context.Background()); err == nil {
		t.Fatal("expected error when the lapsed-paid scan fails")
	}
}

func TestHandler_Run_SaveFailureIsolatedAndContinues(t *testing.T) {
	t.Parallel()
	// The first profile's Save fails; its Auth0 sync must be skipped and it must
	// not be counted, but the cycle must continue to downgrade the second profile.
	finder := &fakeFinder{lapsed: []*profiles.UserProfile{lapsedProfile(t, "auth0|fails"), lapsedProfile(t, "auth0|ok")}}
	saver := newFakeSaver()
	saver.saveErr["auth0|fails"] = errors.New("upsert conflict")
	auth0 := newFakeAuth0()
	h := newHandler(finder, saver, auth0, time.Now())

	downgraded, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if downgraded != 1 {
		t.Errorf("downgraded count: got %d, want 1 (the failing profile is skipped)", downgraded)
	}
	// A failed Save must not reach Auth0 for that profile.
	if _, synced := auth0.synced["auth0|fails"]; synced {
		t.Error("auth0 must not be synced when the Cosmos save failed")
	}
	// The healthy profile still gets downgraded and synced.
	if saver.saved["auth0|ok"] != profiles.TierFree || auth0.synced["auth0|ok"] != "Free" {
		t.Errorf("healthy profile not fully downgraded: saved=%v synced=%v", saver.saved["auth0|ok"], auth0.synced["auth0|ok"])
	}
}

func TestHandler_Run_Auth0FailureIsolatedAndContinues(t *testing.T) {
	t.Parallel()
	// The first profile's Auth0 sync fails after its Cosmos save succeeded; it is
	// not counted, but the cycle continues to the second profile.
	finder := &fakeFinder{lapsed: []*profiles.UserProfile{lapsedProfile(t, "auth0|a0fail"), lapsedProfile(t, "auth0|ok")}}
	saver := newFakeSaver()
	auth0 := newFakeAuth0()
	auth0.syncErr["auth0|a0fail"] = errors.New("auth0 5xx")
	h := newHandler(finder, saver, auth0, time.Now())

	downgraded, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if downgraded != 1 {
		t.Errorf("downgraded count: got %d, want 1 (the auth0-failing profile is skipped)", downgraded)
	}
	if saver.saved["auth0|ok"] != profiles.TierFree || auth0.synced["auth0|ok"] != "Free" {
		t.Errorf("healthy profile not fully downgraded: saved=%v synced=%v", saver.saved["auth0|ok"], auth0.synced["auth0|ok"])
	}
}
