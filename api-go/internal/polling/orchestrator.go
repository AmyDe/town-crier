package polling

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// triggerReceiver destructively receives one poll trigger in receive-and-delete
// mode. ReceiveTrigger reports whether a message was consumed; there is no
// settle (Complete/Abandon) — the message is gone on receive (ADR 0024).
type triggerReceiver interface {
	ReceiveTrigger(ctx context.Context) (received bool, err error)
}

// triggerPublisher publishes the next trigger with a server-side scheduled
// enqueue time. *servicebus.Client satisfies it via PublishAt.
type triggerPublisher interface {
	PublishAt(ctx context.Context, scheduledEnqueueTime time.Time, body []byte) error
}

// leaseStore is the consumer-side slice of the polling lease the orchestrator
// needs. *LeaseStore satisfies it.
type leaseStore interface {
	TryAcquire(ctx context.Context, ttl time.Duration) (LeaseAcquireResult, error)
	Release(ctx context.Context, handle LeaseHandle) LeaseReleaseOutcome
}

// cycleHandler runs one ingestion cycle. *PollPlanItHandler satisfies it.
type cycleHandler interface {
	Handle(ctx context.Context) (PollPlanItResult, error)
}

// nextRunScheduler computes the next trigger's enqueue time. *NextRunScheduler
// satisfies it.
type nextRunScheduler interface {
	ComputeNextRun(reason TerminationReason, retryAfter *time.Duration, now time.Time) time.Time
}

// OrchestratorOptions tune the orchestrator's lease behaviour. LeaseTTL is the
// TTL requested on acquire — it MUST exceed the handler's worst-case runtime so
// the lease cannot expire mid-handler and let a peer start a duplicate cycle
// (default 4.5m vs the 4m handler budget). LeaseAcquireRetryDelay is the single
// retry pause when the lease is briefly held (e.g. by the bootstrap); zero skips
// the pause.
type OrchestratorOptions struct {
	LeaseTTL               time.Duration
	LeaseAcquireRetryDelay time.Duration
}

// OrchestratorRunResult is the outcome of one orchestrator run. PollResult is
// non-nil only when a trigger was received and the handler ran. Mirrors .NET
// PollTriggerOrchestratorRunResult.
type OrchestratorRunResult struct {
	MessageReceived  bool
	PublishedNext    bool
	LeaseUnavailable bool
	PollResult       *PollPlanItResult
}

// Orchestrator glues the Service Bus trigger queue to the ingestion handler under
// the receive-and-delete + publish-after-consume model (ADR 0024 amendment). The
// ordering is acquire lease → receive → handler → publish → release: the lease is
// acquired before the destructive receive so no other actor mutates the queue
// during the critical section, and the next trigger is published before the lease
// is released so a single in-flight trigger is preserved. There is no Service Bus
// settle — a crash anywhere between receive and publish pauses the chain until
// the safety-net bootstrap re-seeds. This is the faithful port of .NET
// PollTriggerOrchestrator.RunOnceAsync.
type Orchestrator struct {
	handler   cycleHandler
	receiver  triggerReceiver
	publisher triggerPublisher
	lease     leaseStore
	scheduler nextRunScheduler
	opts      OrchestratorOptions
	now       func() time.Time
	sleep     func(ctx context.Context, d time.Duration) error
	logger    *slog.Logger
}

// NewOrchestrator wires the orchestrator. now and the internal sleep are injected
// for deterministic tests; production passes time.Now and a context-aware sleep.
func NewOrchestrator(
	handler cycleHandler,
	receiver triggerReceiver,
	publisher triggerPublisher,
	lease leaseStore,
	scheduler nextRunScheduler,
	opts OrchestratorOptions,
	now func() time.Time,
	logger *slog.Logger,
) *Orchestrator {
	return &Orchestrator{
		handler:   handler,
		receiver:  receiver,
		publisher: publisher,
		lease:     lease,
		scheduler: scheduler,
		opts:      opts,
		now:       now,
		sleep:     sleepUntil,
		logger:    logger,
	}
}

// triggerPayload is the next-trigger body. It carries only a diagnostic
// timestamp — the message is a "run once" tick — matching .NET's
// PollTriggerPayload (publishedAtUtc).
type triggerPayload struct {
	PublishedAtUTC string `json:"publishedAtUtc"`
}

// RunOnce performs one orchestrated poll cycle and returns its outcome. It
// returns an error only when the ingestion handler itself errors; expected
// states (lease held, empty queue) surface via the result.
func (o *Orchestrator) RunOnce(ctx context.Context) (OrchestratorRunResult, error) {
	acquire, err := o.acquireWithRetry(ctx)
	if err != nil {
		return OrchestratorRunResult{}, err
	}
	if !acquire.Acquired {
		// Held by a peer or a transient acquire failure: exit cleanly without
		// touching the queue. KEDA / the next trigger re-runs when the peer
		// releases. This is the no-dual-run guarantee — only the lease holder
		// polls PlanIt.
		o.logger.WarnContext(ctx, "polling lease unavailable; exiting without polling",
			"held", acquire.Held, "transient", acquire.TransientErr != nil)
		return OrchestratorRunResult{LeaseUnavailable: true}, nil
	}

	// Release is deferred so the lease is always relinquished, but the next
	// trigger is still published before this returns (publish happens inside the
	// body, release in the defer) — preserving publish-before-release ordering.
	defer func() {
		if outcome := o.lease.Release(ctx, acquire.Handle); outcome == LeasePreconditionFailed {
			o.logger.WarnContext(ctx, "polling lease release returned precondition-failed; TTL is the backstop")
		}
	}()

	received, err := o.receiver.ReceiveTrigger(ctx)
	if err != nil {
		return OrchestratorRunResult{}, err
	}
	if !received {
		o.logger.InfoContext(ctx, "poll trigger queue empty; exiting cleanly (bootstrap will re-seed)")
		return OrchestratorRunResult{}, nil
	}

	result, err := o.handler.Handle(ctx)
	if err != nil {
		// The trigger is already destructively consumed and the cycle failed; do
		// NOT publish a next trigger from a failed cycle — the safety-net bootstrap
		// recovers the chain. Surface the error for the worker's exit code.
		return OrchestratorRunResult{MessageReceived: true}, err
	}

	nextRun := o.scheduler.ComputeNextRun(result.TerminationReason, result.RetryAfter, o.now().UTC())
	body, _ := json.Marshal(triggerPayload{PublishedAtUTC: o.now().UTC().Format(time.RFC3339Nano)})
	if err := o.publisher.PublishAt(ctx, nextRun, body); err != nil {
		return OrchestratorRunResult{MessageReceived: true, PollResult: &result}, err
	}

	return OrchestratorRunResult{
		MessageReceived: true,
		PublishedNext:   true,
		PollResult:      &result,
	}, nil
}

// acquireWithRetry attempts to acquire the lease, retrying once after a short
// pause to absorb the common case where the bootstrap briefly holds it. Mirrors
// .NET's single-retry path.
func (o *Orchestrator) acquireWithRetry(ctx context.Context) (LeaseAcquireResult, error) {
	res, _ := o.lease.TryAcquire(ctx, o.opts.LeaseTTL)
	if res.Acquired {
		return res, nil
	}
	if o.opts.LeaseAcquireRetryDelay > 0 {
		if err := o.sleep(ctx, o.opts.LeaseAcquireRetryDelay); err != nil {
			return LeaseAcquireResult{}, err
		}
	}
	res, _ = o.lease.TryAcquire(ctx, o.opts.LeaseTTL)
	return res, nil
}

// sleepUntil waits d, returning early with the context error if cancelled.
func sleepUntil(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
