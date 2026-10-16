package main

import "context"

type sweepCycle interface {
	Run(ctx context.Context) (int, error)
}

type deliveryWaiter interface {
	Wait()
}

// sweepRunner runs the sweep and then joins in-flight webhook deliveries: the
// job exits as soon as Run returns, which would otherwise drop them.
type sweepRunner struct {
	cycle    sweepCycle
	notifier deliveryWaiter
}

func (s sweepRunner) Run(ctx context.Context) (int, error) {
	downgraded, err := s.cycle.Run(ctx)
	s.notifier.Wait()
	return downgraded, err
}
