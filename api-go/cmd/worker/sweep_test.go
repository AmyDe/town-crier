package main

import (
	"context"
	"testing"
)

type recordingCycle struct{ events *[]string }

func (c recordingCycle) Run(context.Context) (int, error) {
	*c.events = append(*c.events, "run")
	return 3, nil
}

type recordingWaiter struct{ events *[]string }

func (w recordingWaiter) Wait() { *w.events = append(*w.events, "wait") }

func TestSweepRunner_WaitsForDeliveriesAfterRun(t *testing.T) {
	t.Parallel()
	var events []string
	r := sweepRunner{cycle: recordingCycle{&events}, notifier: recordingWaiter{&events}}

	n, err := r.Run(context.Background())

	if err != nil || n != 3 {
		t.Fatalf("Run = %d, %v", n, err)
	}
	if len(events) != 2 || events[0] != "run" || events[1] != "wait" {
		t.Errorf("events = %v, want [run wait]", events)
	}
}
