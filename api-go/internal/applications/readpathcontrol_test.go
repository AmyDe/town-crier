package applications

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeReadPathStore struct {
	row     ReadPathControl
	found   bool
	getErr  error
	setErr  error
	setRows []ReadPathControl
}

func (f *fakeReadPathStore) Get(context.Context) (ReadPathControl, bool, error) {
	return f.row, f.found, f.getErr
}

func (f *fakeReadPathStore) Set(_ context.Context, c ReadPathControl) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.row, f.found = c, true
	f.setRows = append(f.setRows, c)
	return nil
}

func boolPtr(b bool) *bool { return &b }

func TestReadPathSwitch_StateDefaultsEverySwitchOff(t *testing.T) {
	t.Parallel()
	sw := NewReadPathSwitch(&fakeReadPathStore{})

	st, err := sw.State(context.Background())

	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if st.ThinSync || st.ThinList || st.ThinMap {
		t.Errorf("switches = %+v, want all off", st.ReadPathSwitches)
	}
	if st.Source != ReadPathSourceDefault || st.UpdatedAt != nil || st.Reason != "" {
		t.Errorf("state = %+v, want default source and no stamp", st)
	}
}

func TestReadPathSwitch_SetChangesOnlyTheGivenSwitches(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	store := &fakeReadPathStore{found: true, row: ReadPathControl{ReadPathSwitches: ReadPathSwitches{ThinSync: true, ThinMap: true}, Reason: "old", UpdatedAt: at.Add(-time.Hour)}}
	sw := NewReadPathSwitch(store)

	st, err := sw.Set(context.Background(), ReadPathPatch{ThinList: boolPtr(true), ThinMap: boolPtr(false)}, "stage 3", at)

	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	want := ReadPathSwitches{ThinSync: true, ThinList: true, ThinMap: false}
	if st.ReadPathSwitches != want {
		t.Errorf("switches = %+v, want %+v", st.ReadPathSwitches, want)
	}
	if st.Source != ReadPathSourceSet || st.Reason != "stage 3" || st.UpdatedAt == nil || !st.UpdatedAt.Equal(at) {
		t.Errorf("state = %+v", st)
	}
}

func TestReadPathSwitch_SetFromNothingStoredStartsAllOff(t *testing.T) {
	t.Parallel()
	store := &fakeReadPathStore{}
	sw := NewReadPathSwitch(store)

	if _, err := sw.Set(context.Background(), ReadPathPatch{ThinSync: boolPtr(true)}, "", time.Now()); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got := store.setRows[0].ReadPathSwitches
	if want := (ReadPathSwitches{ThinSync: true}); got != want {
		t.Errorf("stored = %+v, want %+v", got, want)
	}
}

func TestReadPathSwitch_StoreErrorsAreWrapped(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")

	_, err := NewReadPathSwitch(&fakeReadPathStore{getErr: boom}).State(context.Background())
	if !errors.Is(err, boom) {
		t.Errorf("State() error = %v, want wrapped boom", err)
	}

	_, err = NewReadPathSwitch(&fakeReadPathStore{setErr: boom}).Set(context.Background(), ReadPathPatch{ThinSync: boolPtr(true)}, "", time.Now())
	if !errors.Is(err, boom) {
		t.Errorf("Set() error = %v, want wrapped boom", err)
	}
}

type countingReadPathStore struct {
	fakeReadPathStore
	gets int
}

func (c *countingReadPathStore) Get(ctx context.Context) (ReadPathControl, bool, error) {
	c.gets++
	return c.fakeReadPathStore.Get(ctx)
}

func TestCachedReadPath_DefaultsOffWithNoRow(t *testing.T) {
	t.Parallel()
	c := NewCachedReadPath(&fakeReadPathStore{}, time.Now)

	if c.ThinList(context.Background()) {
		t.Error("ThinList() = true, want off with no stored row")
	}
}

func TestCachedReadPath_ReadFailureMeansOff(t *testing.T) {
	t.Parallel()
	store := &fakeReadPathStore{found: true, row: ReadPathControl{ReadPathSwitches: ReadPathSwitches{ThinList: true}}, getErr: errors.New("db down")}
	c := NewCachedReadPath(store, time.Now)

	if c.ThinList(context.Background()) {
		t.Error("ThinList() = true, want off on a read failure")
	}
}

func TestCachedReadPath_ReportsStoredSwitch(t *testing.T) {
	t.Parallel()
	store := &fakeReadPathStore{found: true, row: ReadPathControl{ReadPathSwitches: ReadPathSwitches{ThinList: true}}}
	c := NewCachedReadPath(store, time.Now)

	if !c.ThinList(context.Background()) {
		t.Error("ThinList() = false, want on")
	}
}

func TestCachedReadPath_CacheExpiresAfterThirtySeconds(t *testing.T) {
	t.Parallel()
	store := &countingReadPathStore{}
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	c := NewCachedReadPath(store, func() time.Time { return now })
	ctx := context.Background()

	c.ThinList(ctx)
	store.row, store.found = ReadPathControl{ReadPathSwitches: ReadPathSwitches{ThinList: true}}, true

	now = now.Add(29 * time.Second)
	if c.ThinList(ctx) {
		t.Error("ThinList() = true at 29s, want the cached off")
	}
	if store.gets != 1 {
		t.Errorf("store reads = %d at 29s, want 1", store.gets)
	}

	now = now.Add(time.Second)
	if !c.ThinList(ctx) {
		t.Error("ThinList() = false at 30s, want the refreshed on")
	}
	if store.gets != 2 {
		t.Errorf("store reads = %d at 30s, want 2", store.gets)
	}
}

func TestCachedReadPath_ThinMapDefaultsOffAndReadFailureMeansOff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if NewCachedReadPath(&fakeReadPathStore{}, time.Now).ThinMap(ctx) {
		t.Error("ThinMap() = true with no stored row, want off")
	}
	failing := &fakeReadPathStore{found: true, row: ReadPathControl{ReadPathSwitches: ReadPathSwitches{ThinMap: true}}, getErr: errors.New("db down")}
	if NewCachedReadPath(failing, time.Now).ThinMap(ctx) {
		t.Error("ThinMap() = true on a read failure, want off")
	}
	stored := &fakeReadPathStore{found: true, row: ReadPathControl{ReadPathSwitches: ReadPathSwitches{ThinMap: true}}}
	c := NewCachedReadPath(stored, time.Now)
	if !c.ThinMap(ctx) {
		t.Error("ThinMap() = false, want on")
	}
	if c.ThinList(ctx) {
		t.Error("ThinList() = true, want independent of ThinMap")
	}
}

func TestCachedReadPath_ThinMapCacheExpiresAfterThirtySeconds(t *testing.T) {
	t.Parallel()
	store := &countingReadPathStore{}
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	c := NewCachedReadPath(store, func() time.Time { return now })
	ctx := context.Background()

	c.ThinMap(ctx)
	store.row, store.found = ReadPathControl{ReadPathSwitches: ReadPathSwitches{ThinMap: true}}, true

	now = now.Add(29 * time.Second)
	if c.ThinMap(ctx) || store.gets != 1 {
		t.Errorf("at 29s: ThinMap = true or reads = %d, want cached off with 1 read", store.gets)
	}
	now = now.Add(time.Second)
	if !c.ThinMap(ctx) || store.gets != 2 {
		t.Errorf("at 30s: ThinMap = false or reads = %d, want refreshed on with 2 reads", store.gets)
	}
}
