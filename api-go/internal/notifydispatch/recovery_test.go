package notifydispatch

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/devicetokens"
	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// fakePending models the notifications store's recovery queries: rows older than
// the cutoff expire, the rest are returned oldest first up to the limit.
type fakePending struct {
	rows       []notifications.PendingPush
	expired    []string
	cutoffs    []time.Time
	limits     []int
	expireErr  error
	pendingErr error
}

func (f *fakePending) ExpirePendingPushes(_ context.Context, cutoff time.Time) (int, error) {
	if f.expireErr != nil {
		return 0, f.expireErr
	}
	n := 0
	kept := f.rows[:0:0]
	for _, r := range f.rows {
		if r.CreatedAt.Before(cutoff) {
			f.expired = append(f.expired, r.ID)
			n++
			continue
		}
		kept = append(kept, r)
	}
	f.rows = kept
	return n, nil
}

func (f *fakePending) PendingPushes(_ context.Context, cutoff time.Time, limit int) ([]notifications.PendingPush, error) {
	f.cutoffs = append(f.cutoffs, cutoff)
	f.limits = append(f.limits, limit)
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	var out []notifications.PendingPush
	for _, r := range f.rows {
		if !r.CreatedAt.Before(cutoff) {
			out = append(out, r)
		}
	}
	return out, nil
}

var recoverNow = time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)

func pendingRow(id, userID string, zone *string, created time.Time, attempted *time.Time) notifications.PendingPush {
	n := queuedNotif(id)
	n.UserID = userID
	n.WatchZoneID = zone
	n.CreatedAt = created
	return notifications.PendingPush{DigestNotification: n, AttemptedAt: attempted}
}

type recoveryHarness struct {
	c       *PushCoalescer
	pending *fakePending
	push    *fakePush
	marker  *fakeMarker
	profs   *fakeProfiles
}

func newRecoveryHarness(t *testing.T, rows ...notifications.PendingPush) *recoveryHarness {
	t.Helper()
	push := &fakePush{}
	marker := &fakeMarker{}
	devs := &fakeDevices{byUser: map[string][]devicetokens.DeviceRegistration{
		"auth0|alice": {{Token: "tok-1"}},
		"auth0|bob":   {{Token: "tok-2"}},
	}}
	pro := profileWithTier(t, "auth0|alice", profiles.TierPro)
	pro.Preferences.PushEnabled = true
	bob := profileWithTier(t, "auth0|bob", profiles.TierPro)
	bob.Preferences.PushEnabled = true
	profs := &fakeProfiles{byID: map[string]*profiles.UserProfile{"auth0|alice": pro, "auth0|bob": bob}}
	pending := &fakePending{rows: rows}
	c := NewPushCoalescer(devs, &fakeState{}, NewPlatformDispatcher(push, &fakePush{}, testLogger(t)),
		&fakeZoneNames{}, marker, func() time.Time { return recoverNow }, testLogger(t)).
		WithRecovery(pending, profs)
	return &recoveryHarness{c: c, pending: pending, push: push, marker: marker, profs: profs}
}

func strp(s string) *string { return &s }

func TestPushCoalescer_Recover_StoppedRunSendsPendingOnce(t *testing.T) {
	t.Parallel()
	created := recoverNow.Add(-time.Hour)
	h := newRecoveryHarness(t,
		pendingRow("n-1", "auth0|alice", strp("z1"), created, nil),
		pendingRow("n-2", "auth0|alice", strp("z1"), created, nil),
	)

	if err := h.c.Recover(context.Background(), recoverNow); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if err := h.c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if h.push.calls != 1 {
		t.Fatalf("send calls = %d, want 1 summary push", h.push.calls)
	}
	if got := h.marker.statuses[notifications.PushStatusSent]; len(got) != 1 || len(got[0]) != 2 {
		t.Errorf("sent marks = %v, want one mark of 2 ids", got)
	}
	if err := h.c.Flush(context.Background()); err != nil {
		t.Fatalf("second Flush: %v", err)
	}
	if h.push.calls != 1 {
		t.Errorf("second flush resent: calls = %d", h.push.calls)
	}
}

func TestPushCoalescer_Recover_MergesWithCurrentRunByNotificationID(t *testing.T) {
	t.Parallel()
	created := recoverNow.Add(-time.Minute)
	h := newRecoveryHarness(t,
		pendingRow("n-1", "auth0|alice", strp("z1"), created, nil),
		pendingRow("n-old", "auth0|alice", strp("z1"), recoverNow.Add(-2*time.Hour), nil),
	)
	n1 := queuedNotif("n-1")
	n1.WatchZoneID = strp("z1")
	h.c.Add("auth0|alice", n1)

	if err := h.c.Recover(context.Background(), recoverNow); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if err := h.c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if h.push.calls != 1 {
		t.Fatalf("send calls = %d, want 1 merged push", h.push.calls)
	}
	got := h.marker.statuses[notifications.PushStatusSent]
	if len(got) != 1 {
		t.Fatalf("sent marks = %v, want one", got)
	}
	slices.Sort(got[0])
	if !slices.Equal(got[0], []string{"n-1", "n-old"}) {
		t.Errorf("sent ids = %v, want [n-1 n-old] with no duplicate", got[0])
	}
}

func TestPushCoalescer_Recover_AttemptedRowsRegroupedWithSameCollapseID(t *testing.T) {
	t.Parallel()
	created := recoverNow.Add(-time.Hour)
	first := recoverNow.Add(-50 * time.Minute)
	second := recoverNow.Add(-20 * time.Minute)
	h := newRecoveryHarness(t,
		pendingRow("a-1", "auth0|alice", strp("z1"), created, &first),
		pendingRow("a-2", "auth0|alice", strp("z1"), created, &first),
		pendingRow("b-1", "auth0|alice", strp("z1"), created, &second),
		pendingRow("n-3", "auth0|alice", strp("z1"), created, nil),
	)

	if err := h.c.Recover(context.Background(), recoverNow); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if err := h.c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if h.push.calls != 3 {
		t.Fatalf("send calls = %d, want 3 (two attempted sets plus the fresh row)", h.push.calls)
	}
	for _, want := range []string{pushCollapseID([]string{"a-1", "a-2"}), "b-1", "n-3"} {
		if !slices.Contains(h.push.collapseIDs, want) {
			t.Errorf("collapse IDs = %v, missing %q", h.push.collapseIDs, want)
		}
	}
}

func TestPushCoalescer_Recover_IneligibleUserMarkedSkipped(t *testing.T) {
	t.Parallel()
	created := recoverNow.Add(-time.Hour)
	h := newRecoveryHarness(t,
		pendingRow("n-1", "auth0|alice", nil, created, nil),
		pendingRow("n-2", "auth0|bob", nil, created, nil),
		pendingRow("n-3", "auth0|ghost", nil, created, nil),
	)
	h.profs.byID["auth0|alice"].Preferences.PushEnabled = false
	h.profs.byID["auth0|bob"].Tier = profiles.TierFree

	if err := h.c.Recover(context.Background(), recoverNow); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if err := h.c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if h.push.calls != 0 {
		t.Errorf("send calls = %d, want 0", h.push.calls)
	}
	var skipped []string
	for _, ids := range h.marker.statuses[notifications.PushStatusSkipped] {
		skipped = append(skipped, ids...)
	}
	slices.Sort(skipped)
	if !slices.Equal(skipped, []string{"n-1", "n-2", "n-3"}) {
		t.Errorf("skipped = %v, want n-1 n-2 n-3", skipped)
	}
}

func TestPushCoalescer_Recover_ExpiresAtTwelveHourEdge(t *testing.T) {
	t.Parallel()
	h := newRecoveryHarness(t,
		pendingRow("edge", "auth0|alice", nil, recoverNow.Add(-12*time.Hour), nil),
		pendingRow("past", "auth0|alice", nil, recoverNow.Add(-12*time.Hour-time.Second), nil),
	)

	if err := h.c.Recover(context.Background(), recoverNow); err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if err := h.c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if !slices.Equal(h.pending.expired, []string{"past"}) {
		t.Errorf("expired = %v, want [past]", h.pending.expired)
	}
	if got := h.marker.statuses[notifications.PushStatusSent]; len(got) != 1 || !slices.Equal(got[0], []string{"edge"}) {
		t.Errorf("sent = %v, want [[edge]]", got)
	}
	if !slices.Equal(h.pending.limits, []int{recoveryBatchLimit}) {
		t.Errorf("limits = %v, want [%d]", h.pending.limits, recoveryBatchLimit)
	}
}

func TestPushCoalescer_Recover_StoreFailureIsReturnedAndNothingQueued(t *testing.T) {
	t.Parallel()
	h := newRecoveryHarness(t, pendingRow("n-1", "auth0|alice", nil, recoverNow.Add(-time.Hour), nil))
	h.pending.pendingErr = errors.New("db down")

	if err := h.c.Recover(context.Background(), recoverNow); err == nil {
		t.Fatal("Recover returned nil, want the store error")
	}
	if err := h.c.Flush(context.Background()); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if h.push.calls != 0 {
		t.Errorf("send calls = %d, want 0", h.push.calls)
	}
}
