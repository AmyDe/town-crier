package polling

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakePollControlStore struct {
	c      *PollControl
	getErr error
}

func (f *fakePollControlStore) Get(context.Context) (PollControl, bool, error) {
	if f.getErr != nil {
		return PollControl{}, false, f.getErr
	}
	if f.c == nil {
		return PollControl{}, false, nil
	}
	return *f.c, true, nil
}

func (f *fakePollControlStore) Set(_ context.Context, c PollControl) error {
	f.c = &c
	return nil
}

func TestSwitch_NoRowUsesDefault(t *testing.T) {
	t.Parallel()
	for _, def := range []bool{true, false} {
		st, err := NewSwitch(&fakePollControlStore{}, def).State(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if st.Enabled != def || st.Source != SwitchSourceDefault || st.UpdatedAt != nil {
			t.Fatalf("default %v: state = %+v", def, st)
		}
	}
}

func TestSwitch_SetOverridesDefault(t *testing.T) {
	t.Parallel()
	s := NewSwitch(&fakePollControlStore{}, true)
	at := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)

	st, err := s.Set(context.Background(), false, "dev trial", at)

	if err != nil {
		t.Fatal(err)
	}
	if st.Enabled || st.Source != SwitchSourceSet || st.Reason != "dev trial" || st.UpdatedAt == nil || !st.UpdatedAt.Equal(at) {
		t.Fatalf("state = %+v", st)
	}
	if on, err := s.Enabled(context.Background()); err != nil || on {
		t.Fatalf("Enabled = %v, %v", on, err)
	}
}

func TestSwitch_ReadErrorIsNotEnabled(t *testing.T) {
	t.Parallel()
	s := NewSwitch(&fakePollControlStore{getErr: errors.New("db down")}, true)

	on, err := s.Enabled(context.Background())

	if err == nil || on {
		t.Fatalf("Enabled = %v, %v; want false and an error", on, err)
	}
}
