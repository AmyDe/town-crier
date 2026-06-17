package polling

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// fakeStateItems is a hand-written double for the poll-state store's Cosmos
// accessor. It keeps documents in a map keyed by id and supports a
// cross-partition "all poll-state docs" query.
type fakeStateItems struct {
	docs        map[string][]byte
	readErr     error
	upsertErr   error
	queryErr    error
	upsertCalls int
}

func newFakeStateItems() *fakeStateItems {
	return &fakeStateItems{docs: map[string][]byte{}}
}

func (f *fakeStateItems) ReadItem(_ context.Context, _, id string) ([]byte, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	body, ok := f.docs[id]
	if !ok {
		return nil, &azcore.ResponseError{StatusCode: http.StatusNotFound}
	}
	return body, nil
}

func (f *fakeStateItems) UpsertItem(_ context.Context, _ string, item []byte) error {
	f.upsertCalls++
	if f.upsertErr != nil {
		return f.upsertErr
	}
	var doc pollStateDocument
	if err := json.Unmarshal(item, &doc); err != nil {
		return err
	}
	f.docs[doc.ID] = item
	return nil
}

func (f *fakeStateItems) QueryItemsCrossPartition(_ context.Context, _ string, _ map[string]any) ([][]byte, error) {
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	out := make([][]byte, 0, len(f.docs))
	for _, body := range f.docs {
		out = append(out, body)
	}
	return out, nil
}

func TestPollStateStore_SaveThenGetRoundTrips(t *testing.T) {
	t.Parallel()
	items := newFakeStateItems()
	store := NewPollStateStore(items)
	ctx := context.Background()

	lastPoll := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	hwm := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	cursor := &PollCursor{DifferentStart: time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC), NextPage: 3, KnownTotal: platform.Ptr(250)}

	if err := store.Save(ctx, 99, lastPoll, hwm, cursor); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, ok, err := store.Get(ctx, 99)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: expected state to exist")
	}
	if !got.LastPollTime.Equal(lastPoll) || !got.HighWaterMark.Equal(hwm) {
		t.Errorf("times: lastPoll=%v hwm=%v", got.LastPollTime, got.HighWaterMark)
	}
	if got.Cursor == nil || got.Cursor.NextPage != 3 || got.Cursor.KnownTotal == nil || *got.Cursor.KnownTotal != 250 {
		t.Errorf("cursor: %+v", got.Cursor)
	}
	if !got.Cursor.DifferentStart.Equal(cursor.DifferentStart) {
		t.Errorf("cursor different_start: got %v, want %v", got.Cursor.DifferentStart, cursor.DifferentStart)
	}
}

func TestPollStateStore_GetMissingReturnsNotOK(t *testing.T) {
	t.Parallel()
	store := NewPollStateStore(newFakeStateItems())
	_, ok, err := store.Get(context.Background(), 7)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ok {
		t.Error("expected ok=false for missing state")
	}
}

func TestPollStateStore_SaveWithoutCursorClearsIt(t *testing.T) {
	t.Parallel()
	items := newFakeStateItems()
	store := NewPollStateStore(items)
	ctx := context.Background()
	now := time.Now().UTC()

	// Save with a cursor, then save again without one — the second read must show
	// no active cursor.
	_ = store.Save(ctx, 5, now, now, &PollCursor{DifferentStart: now, NextPage: 2})
	if err := store.Save(ctx, 5, now, now, nil); err != nil {
		t.Fatalf("Save (clear cursor): %v", err)
	}
	got, _, err := store.Get(ctx, 5)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Cursor != nil {
		t.Errorf("cursor should be cleared, got %+v", got.Cursor)
	}
}

func TestPollStateStore_GetLeastRecentlyPolledOrdersNeverPolledFirstThenByLastPollTime(t *testing.T) {
	t.Parallel()
	items := newFakeStateItems()
	store := NewPollStateStore(items)
	ctx := context.Background()

	t0 := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	// 100 polled most recently, 200 polled longest ago. 300 is never-polled.
	_ = store.Save(ctx, 100, t0.Add(2*time.Hour), t0, nil)
	_ = store.Save(ctx, 200, t0, t0, nil)

	res, err := store.GetLeastRecentlyPolled(ctx, []int{100, 200, 300})
	if err != nil {
		t.Fatalf("GetLeastRecentlyPolled: %v", err)
	}
	// Never-polled (300) first, then oldest-polled (200), then newest (100).
	want := []int{300, 200, 100}
	if len(res.AuthorityIDs) != len(want) {
		t.Fatalf("ids: got %v, want %v", res.AuthorityIDs, want)
	}
	for i := range want {
		if res.AuthorityIDs[i] != want[i] {
			t.Errorf("order[%d]: got %d, want %d (full=%v)", i, res.AuthorityIDs[i], want[i], res.AuthorityIDs)
		}
	}
	if res.NeverPolledCount != 1 {
		t.Errorf("never-polled count: got %d, want 1", res.NeverPolledCount)
	}
}

func TestPollStateStore_GetLeastRecentlyPolledEmptyCandidates(t *testing.T) {
	t.Parallel()
	store := NewPollStateStore(newFakeStateItems())
	res, err := store.GetLeastRecentlyPolled(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetLeastRecentlyPolled: %v", err)
	}
	if len(res.AuthorityIDs) != 0 || res.NeverPolledCount != 0 {
		t.Errorf("empty candidates should yield empty result, got %+v", res)
	}
}

func TestPollStateStore_GetSurfacesNonNotFoundError(t *testing.T) {
	t.Parallel()
	items := newFakeStateItems()
	items.readErr = errors.New("cosmos down")
	store := NewPollStateStore(items)
	if _, _, err := store.Get(context.Background(), 1); err == nil {
		t.Error("expected error from underlying read failure")
	}
}
