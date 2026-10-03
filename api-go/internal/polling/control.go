package polling

import (
	"context"
	"fmt"
	"time"
)

// Switch sources reported by SwitchState.Source.
const (
	SwitchSourceDefault = "default"
	SwitchSourceSet     = "set"
)

// PollControl is the stored polling switch of one environment.
type PollControl struct {
	Enabled   bool
	Reason    string
	UpdatedAt time.Time
}

// SwitchState is the effective polling switch. Source is SwitchSourceDefault
// when nothing is stored and the environment default applies; Reason and
// UpdatedAt are then empty.
type SwitchState struct {
	Enabled   bool
	Source    string
	Reason    string
	UpdatedAt *time.Time
}

type pollControlStore interface {
	Get(ctx context.Context) (PollControl, bool, error)
	Set(ctx context.Context, c PollControl) error
}

// Switch is the per-environment on/off control for PlanIt polling. Only one
// environment may poll at a time, because PlanIt treats prod and dev as one
// client (POLLING.md).
type Switch struct {
	store pollControlStore
	def   bool
}

// NewSwitch wires a Switch. def applies until a value is stored
// (POLLING_ENABLED_DEFAULT).
func NewSwitch(store pollControlStore, def bool) *Switch {
	return &Switch{store: store, def: def}
}

// Enabled reports whether this environment may poll PlanIt now.
func (s *Switch) Enabled(ctx context.Context) (bool, error) {
	st, err := s.State(ctx)
	if err != nil {
		return false, err
	}
	return st.Enabled, nil
}

// State returns the effective switch.
func (s *Switch) State(ctx context.Context) (SwitchState, error) {
	c, found, err := s.store.Get(ctx)
	if err != nil {
		return SwitchState{}, fmt.Errorf("read poll_control: %w", err)
	}
	if !found {
		return SwitchState{Enabled: s.def, Source: SwitchSourceDefault}, nil
	}
	at := c.UpdatedAt
	return SwitchState{Enabled: c.Enabled, Source: SwitchSourceSet, Reason: c.Reason, UpdatedAt: &at}, nil
}

// Set stores the switch and returns the state read back from the store.
func (s *Switch) Set(ctx context.Context, enabled bool, reason string, at time.Time) (SwitchState, error) {
	if err := s.store.Set(ctx, PollControl{Enabled: enabled, Reason: reason, UpdatedAt: at}); err != nil {
		return SwitchState{}, fmt.Errorf("write poll_control: %w", err)
	}
	return s.State(ctx)
}
