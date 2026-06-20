package profiles

import (
	"context"
	"testing"
	"time"
)

func TestActivityRecorder_DedupesWithin24h(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	base := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	p, _ := NewProfile("auth0|abc", "", base)
	store.byID["auth0|abc"] = p

	rec := NewActivityRecorder(store)

	// A second activity within 24h of the last must NOT trigger a save
	// (WriteDedupeWindow short-circuit).
	if err := rec.RecordActivity(context.Background(), "auth0|abc", base.Add(time.Hour)); err != nil {
		t.Fatalf("RecordActivity: %v", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("activity within 24h should not save, got %d saves", len(store.saved))
	}

	// Beyond 24h, it saves and advances LastActiveAt.
	later := base.Add(25 * time.Hour)
	if err := rec.RecordActivity(context.Background(), "auth0|abc", later); err != nil {
		t.Fatalf("RecordActivity: %v", err)
	}
	if len(store.saved) != 1 {
		t.Fatalf("activity beyond 24h should save once, got %d", len(store.saved))
	}
	if !store.byID["auth0|abc"].LastActiveAt.Equal(later) {
		t.Errorf("LastActiveAt not advanced: got %v, want %v", store.byID["auth0|abc"].LastActiveAt, later)
	}
}

func TestActivityRecorder_UnknownUserIsNoOp(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	rec := NewActivityRecorder(store)

	// A missing profile is not an error and creates nothing — registration only
	// happens via POST /v1/me, never the activity middleware.
	if err := rec.RecordActivity(context.Background(), "auth0|missing", time.Now()); err != nil {
		t.Errorf("RecordActivity for unknown user: got %v, want nil", err)
	}
	if len(store.saved) != 0 {
		t.Errorf("unknown user should not create a profile, got %d saves", len(store.saved))
	}
}

func TestTierLookup_PaidFromCosmosProfile(t *testing.T) {
	t.Parallel()

	store := newFakeStore()
	free, _ := NewProfile("auth0|free", "", time.Now())
	store.byID["auth0|free"] = free
	pro, _ := NewProfile("auth0|pro", "", time.Now())
	pro.Tier = TierPro
	store.byID["auth0|pro"] = pro

	lookup := NewTierLookup(store)

	paid, err := lookup.IsPaidUser(context.Background(), "auth0|pro")
	if err != nil || !paid {
		t.Errorf("pro user: paid=%v err=%v, want paid=true", paid, err)
	}
	paid, err = lookup.IsPaidUser(context.Background(), "auth0|free")
	if err != nil || paid {
		t.Errorf("free user: paid=%v err=%v, want paid=false", paid, err)
	}
}

func TestTierLookup_MissingProfileIsFree(t *testing.T) {
	t.Parallel()

	lookup := NewTierLookup(newFakeStore())
	paid, err := lookup.IsPaidUser(context.Background(), "auth0|missing")
	// A user with no profile is treated as free, not an error — they simply have
	// not registered yet, so the lower limit applies.
	if err != nil {
		t.Errorf("missing profile should not error: %v", err)
	}
	if paid {
		t.Error("missing profile should be treated as free")
	}
}
