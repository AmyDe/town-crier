// Package worker holds the background-worker modes that run as short-lived
// Container Apps Jobs.
package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	mathrand "math/rand/v2"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/polling"
	"github.com/AmyDe/town-crier/api-go/internal/servicebus"
)

// A reseed restarts the chain 5m out, not at the scheduler's 1h natural cadence.
const (
	naturalCadence = 5 * time.Minute
	jitterBound    = 10 * time.Second

	bootstrapLeaseTTL = 1 * time.Minute

	// Absorbs jitter, clock skew and a trigger published late in a deploy.
	parkedTriggerMargin = 15 * time.Minute
)

// JitterBound is excluded because parkedTriggerMargin already covers it.
func maxLegitimateDelay(opts polling.SchedulerOptions) time.Duration {
	longest := opts.NaturalCadence
	for _, d := range [...]time.Duration{
		opts.TimeBoundedCadence,
		opts.RetryAfterCap,
		opts.RateLimitDefault,
		opts.TimeoutCadence,
	} {
		if d > longest {
			longest = d
		}
	}
	return longest
}

// Derived from opts, never a literal, so that a longer scheduler delay can never
// make the bootstrap cancel a trigger the scheduler parked on purpose.
func parkedTriggerThreshold(opts polling.SchedulerOptions) time.Duration {
	return maxLegitimateDelay(opts) + parkedTriggerMargin
}

// triggerQueue is the consumer-side interface the bootstrapper depends on. It
// is satisfied by *servicebus.Client in production and by a hand-written fake
// in tests. Only the methods the bootstrapper uses are declared here: probe
// and seed (original safety net), plus peek, cancel and receive-and-discard
// (the GH#938 PR2 reconciler) and dead-letter drain.
type triggerQueue interface {
	QueueDepth(ctx context.Context) (servicebus.QueueDepth, error)
	PublishAt(ctx context.Context, scheduledEnqueueTime time.Time, body []byte) error
	PeekMessages(ctx context.Context) ([]servicebus.PeekedMessage, error)
	CancelScheduled(ctx context.Context, sequenceNumbers []int64) error
	ReceiveTrigger(ctx context.Context) (bool, error)
	DrainDeadLetters(ctx context.Context) (int, error)
}

// leaseAccess is the consumer-side slice of the Postgres polling lease the
// bootstrapper needs. *polling.PostgresLeaseStore satisfies it. Sharing the same
// lease the orchestrator acquires (poll-sb mode) is what closes the unleased-
// bootstrap fork mechanism: no actor other than the current holder can mutate
// the trigger queue (GH#938 PR1).
type leaseAccess interface {
	TryAcquire(ctx context.Context, ttl time.Duration) (polling.LeaseAcquireResult, error)
	Release(ctx context.Context, handle polling.LeaseHandle) polling.LeaseReleaseOutcome
}

// leaseMetricsRecorder is the consumer-side slice of the metrics registry the
// bootstrapper records the lease-acquired counter on. *metrics.Registry
// satisfies it; nil no-ops, so the counter stays dark until WithLeaseMetrics
// wires a recorder. caller is always "bootstrap" here (the orchestrator records
// its own acquisitions with caller "orchestrator").
type leaseMetricsRecorder interface {
	LeaseAcquired(ctx context.Context, caller string)
}

// BootstrapResult is the outcome of TryBootstrap. Each field is emitted as a
// polling.safety_net.* span attribute.
type BootstrapResult struct {
	// Published reports that a seed trigger was published.
	Published bool
	// ProbeFailed reports that a lease acquire, probe, peek or publish failed.
	ProbeFailed bool
	// LeaseUnavailable reports that a peer held the polling lease, so the queue
	// was not probed.
	LeaseUnavailable bool
	// Reconciled reports that a forked queue was collapsed to one live trigger.
	Reconciled bool
	// ParkedRecovered reports that a lone scheduled trigger due beyond every
	// legitimate scheduler delay was cancelled and its replacement published.
	ParkedRecovered bool
	// ScheduledCancelled is the number of scheduled triggers cancelled. It
	// counts a parked-trigger cancel even when the replacement publish failed.
	ScheduledCancelled int
	// ActiveDiscarded is the number of surplus active triggers discarded while
	// reconciling a fork.
	ActiveDiscarded int
	// DeadLettered is the number of dead-lettered messages drained.
	DeadLettered int
}

// Bootstrapper keeps the poll-trigger queue at one live trigger under the
// shared polling lease, and drains its dead letters.
type Bootstrapper struct {
	queue   triggerQueue
	lease   leaseAccess
	logger  *slog.Logger
	now     func() time.Time
	metrics leaseMetricsRecorder
}

// NewBootstrapper wires the bootstrapper. now is injected so tests can pin time;
// production passes time.Now.
func NewBootstrapper(queue triggerQueue, lease leaseAccess, logger *slog.Logger, now func() time.Time) *Bootstrapper {
	return &Bootstrapper{queue: queue, lease: lease, logger: logger, now: now}
}

// WithLeaseMetrics wires the recorder the bootstrapper records the
// lease-acquired counter on. A post-construction setter (mirroring the
// orchestrator's WithLeaseMetrics) so the existing lease-guard tests are
// unaffected by a missing registry; cmd/worker calls it once after building the
// bootstrapper. Returns the bootstrapper for chaining.
func (b *Bootstrapper) WithLeaseMetrics(rec leaseMetricsRecorder) *Bootstrapper {
	b.metrics = rec
	return b
}

// triggerPayload is the seed message body. It carries only a diagnostic
// timestamp — the message is a "run once now" tick (publishedAtUtc).
type triggerPayload struct {
	PublishedAtUtc string `json:"publishedAtUtc"`
}

// TryBootstrap runs one bootstrap tick. When a peer holds the polling lease it
// does nothing and reports LeaseUnavailable. It absorbs every failure into the
// result, so the returned error is always nil.
func (b *Bootstrapper) TryBootstrap(ctx context.Context) (BootstrapResult, error) {
	acquire, err := b.lease.TryAcquire(ctx, bootstrapLeaseTTL)
	if err != nil {
		b.logger.WarnContext(ctx, "poll-bootstrap lease acquire failed; skipping reseed", "error", err)
		return BootstrapResult{ProbeFailed: true}, nil
	}
	if acquire.TransientErr != nil {
		b.logger.WarnContext(ctx, "poll-bootstrap lease acquire failed; skipping reseed", "error", acquire.TransientErr)
		return BootstrapResult{ProbeFailed: true}, nil
	}
	if !acquire.Acquired {
		b.logger.InfoContext(ctx, "poll-bootstrap skipped; polling lease held")
		return BootstrapResult{LeaseUnavailable: true}, nil
	}

	if b.metrics != nil {
		b.metrics.LeaseAcquired(ctx, "bootstrap")
	}

	defer func() {
		if outcome := b.lease.Release(ctx, acquire.Handle); outcome == polling.LeasePreconditionFailed {
			b.logger.WarnContext(ctx, "poll-bootstrap lease release returned precondition-failed; TTL is the backstop")
		}
	}()

	depth, err := b.queue.QueueDepth(ctx)
	if err != nil {
		b.logger.WarnContext(ctx, "poll-bootstrap probe failed; skipping reseed", "error", err)
		return BootstrapResult{ProbeFailed: true}, nil
	}

	result := b.reconcileToSingleTrigger(ctx, depth)

	// Dead letters never clear themselves, so drain whatever the outcome above.
	drained, drainErr := b.queue.DrainDeadLetters(ctx)
	if drainErr != nil {
		b.logger.WarnContext(ctx, "poll-bootstrap dead-letter drain failed; next cron tick will retry", "error", drainErr)
	} else if drained > 0 {
		b.logger.InfoContext(ctx, "poll-bootstrap drained dead-letter queue", "drained", drained)
	}
	result.DeadLettered = drained

	return result, nil
}

// reconcileToSingleTrigger leaves at most one live (active+scheduled) trigger,
// barring an absorbed failure.
func (b *Bootstrapper) reconcileToSingleTrigger(ctx context.Context, depth servicebus.QueueDepth) BootstrapResult {
	switch triggerCount := depth.TriggerCount(); triggerCount {
	case 0:
		return b.seed(ctx)
	case 1:
		return b.reconcileSingleTrigger(ctx, depth)
	default:
		b.logger.WarnContext(ctx, "poll-bootstrap detected forked trigger chain; reconciling to one",
			"active", depth.ActiveMessageCount,
			"scheduled", depth.ScheduledMessageCount,
			"deadLettered", depth.DeadLetterMessageCount)
		return b.reconcileFork(ctx)
	}
}

func (b *Bootstrapper) reconcileSingleTrigger(ctx context.Context, depth servicebus.QueueDepth) BootstrapResult {
	if depth.ActiveMessageCount > 0 {
		// KEDA runs an active trigger, so leave the queue alone.
		b.logger.InfoContext(ctx, "poll-bootstrap skipped; trigger queue already seeded",
			"active", depth.ActiveMessageCount,
			"scheduled", depth.ScheduledMessageCount,
			"deadLettered", depth.DeadLetterMessageCount)
		return BootstrapResult{}
	}
	return b.recoverParkedTrigger(ctx, depth)
}

// KEDA scales on active messages only, so a lone scheduled trigger parked too
// far out stalls the whole chain until it activates.
func (b *Bootstrapper) recoverParkedTrigger(ctx context.Context, depth servicebus.QueueDepth) BootstrapResult {
	now := b.now()

	peeked, err := b.queue.PeekMessages(ctx)
	if err != nil {
		b.logger.WarnContext(ctx, "poll-bootstrap parked-trigger peek failed; skipping this cycle", "error", err)
		return BootstrapResult{ProbeFailed: true}
	}

	var scheduled []servicebus.PeekedMessage
	for _, m := range peeked {
		if m.State == servicebus.MessageStateScheduled {
			scheduled = append(scheduled, m)
		}
	}
	if len(scheduled) != 1 {
		// The queue changed between probe and peek; do not guess, the next tick
		// re-probes.
		b.logger.WarnContext(ctx, "poll-bootstrap depth probe reported one scheduled trigger but peek disagreed; skipping this cycle",
			"scheduledPeeked", len(scheduled), "totalPeeked", len(peeked))
		return BootstrapResult{}
	}

	msg := scheduled[0]
	threshold := parkedTriggerThreshold(polling.DefaultSchedulerOptions())
	deadline := now.Add(threshold)

	if !msg.ScheduledEnqueueTime.After(deadline) {
		b.logger.InfoContext(ctx, "poll-bootstrap skipped; trigger queue already seeded",
			"active", depth.ActiveMessageCount,
			"scheduled", depth.ScheduledMessageCount,
			"deadLettered", depth.DeadLetterMessageCount,
			"activatesAt", msg.ScheduledEnqueueTime.UTC(),
			"sequenceNumber", msg.SequenceNumber)
		return BootstrapResult{}
	}

	b.logger.WarnContext(ctx, "poll-bootstrap detected a parked trigger beyond every legitimate scheduler delay; cancelling and reseeding",
		"activatesAt", msg.ScheduledEnqueueTime.UTC(),
		"sequenceNumber", msg.SequenceNumber,
		"threshold", threshold)

	// Cancel before publish: a failed cancel keeps the old trigger and a failed
	// publish leaves an empty queue for the next tick, so the chain never forks.
	if err := b.queue.CancelScheduled(ctx, []int64{msg.SequenceNumber}); err != nil {
		b.logger.WarnContext(ctx, "poll-bootstrap cancel of parked trigger failed; leaving it in place",
			"error", err, "sequenceNumber", msg.SequenceNumber)
		return BootstrapResult{}
	}

	result := b.seed(ctx)
	result.ScheduledCancelled = 1
	result.ParkedRecovered = result.Published
	return result
}

// seed publishes one jittered seed trigger for a queue found completely empty.
// Behaviour is unchanged from the pre-PR2 bootstrap.
func (b *Bootstrapper) seed(ctx context.Context) BootstrapResult {
	now := b.now()
	scheduledAt := now.Add(nextSeedDelay())

	body, err := json.Marshal(triggerPayload{PublishedAtUtc: now.UTC().Format(time.RFC3339Nano)})
	if err != nil {
		// json.Marshal of a fixed struct cannot realistically fail; treat any
		// failure as a probe failure for telemetry parity rather than panicking.
		b.logger.WarnContext(ctx, "poll-bootstrap payload marshal failed; skipping reseed", "error", err)
		return BootstrapResult{ProbeFailed: true}
	}

	if err := b.queue.PublishAt(ctx, scheduledAt, body); err != nil {
		b.logger.WarnContext(ctx, "poll-bootstrap publish failed; next cron tick will retry", "error", err)
		return BootstrapResult{ProbeFailed: true}
	}

	b.logger.InfoContext(ctx, "poll-bootstrap published seed trigger", "scheduledAt", scheduledAt.UTC())
	return BootstrapResult{Published: true}
}

// reconcileFork collapses a queue holding more than one live trigger back down
// to exactly one, preferring the latest-activating scheduled message
// (Pre-Resolved Design Decision, GH#938 — it carries the most recent
// Retry-After knowledge, the safest choice for PlanIt). Every other scheduled
// message is cancelled by sequence number; every surplus active message is
// destructively received and discarded. A peek failure is absorbed exactly
// like a depth-probe failure — the next cron tick retries. A cancel or discard
// failure is logged and the count reflects only what actually succeeded; it
// never escalates to a caller-visible error, so a partially-reconciled fork is
// simply retried (and re-peeked) on the next tick.
func (b *Bootstrapper) reconcileFork(ctx context.Context) BootstrapResult {
	peeked, err := b.queue.PeekMessages(ctx)
	if err != nil {
		b.logger.WarnContext(ctx, "poll-bootstrap reconciliation peek failed; skipping this cycle", "error", err)
		return BootstrapResult{ProbeFailed: true}
	}
	if len(peeked) == 0 {
		// The depth probe reported a fork but peek found nothing concrete to act
		// on (e.g. the fork resolved itself between the two calls). Nothing to do
		// this cycle; the next tick re-probes.
		b.logger.WarnContext(ctx, "poll-bootstrap fork detected by depth but peek returned no messages; skipping reconciliation this cycle")
		return BootstrapResult{}
	}

	keep, cancelSeqs, discardCount := pickKeeper(peeked)

	cancelled := 0
	if len(cancelSeqs) > 0 {
		if err := b.queue.CancelScheduled(ctx, cancelSeqs); err != nil {
			b.logger.WarnContext(ctx, "poll-bootstrap cancel of surplus scheduled triggers failed",
				"error", err, "attempted", len(cancelSeqs))
		} else {
			cancelled = len(cancelSeqs)
		}
	}

	discarded := 0
	for range discardCount {
		received, err := b.queue.ReceiveTrigger(ctx)
		if err != nil {
			b.logger.WarnContext(ctx, "poll-bootstrap discard of surplus active trigger failed",
				"error", err, "discardedSoFar", discarded)
			break
		}
		if !received {
			break
		}
		discarded++
	}

	b.logger.InfoContext(ctx, "poll-bootstrap reconciled forked trigger chain",
		"scheduledCancelled", cancelled,
		"activeDiscarded", discarded,
		"keptSequenceNumber", keep.SequenceNumber,
		"keptState", keep.State,
		"keptActivatesAt", keep.ScheduledEnqueueTime.UTC())

	return BootstrapResult{
		Reconciled:         true,
		ScheduledCancelled: cancelled,
		ActiveDiscarded:    discarded,
	}
}

// pickKeeper decides which peeked message survives reconciliation. When any
// scheduled message exists it keeps the latest-activating one (Pre-Resolved
// Design Decision, GH#938) and reports every other scheduled message's
// sequence number for cancellation, plus the count of active messages to
// discard (all of them — a scheduled survivor always wins over an active one).
// When no scheduled message exists it keeps the lowest-sequence-numbered
// active message (the oldest) and reports the rest for discard.
func pickKeeper(peeked []servicebus.PeekedMessage) (keep servicebus.PeekedMessage, cancelScheduled []int64, discardActive int) {
	var scheduled, active []servicebus.PeekedMessage
	for _, m := range peeked {
		if m.State == servicebus.MessageStateScheduled {
			scheduled = append(scheduled, m)
		} else {
			active = append(active, m)
		}
	}

	if len(scheduled) > 0 {
		keepIdx := 0
		for i, m := range scheduled {
			if m.ScheduledEnqueueTime.After(scheduled[keepIdx].ScheduledEnqueueTime) {
				keepIdx = i
			}
		}
		for i, m := range scheduled {
			if i != keepIdx {
				cancelScheduled = append(cancelScheduled, m.SequenceNumber)
			}
		}
		return scheduled[keepIdx], cancelScheduled, len(active)
	}

	keepIdx := 0
	for i, m := range active {
		if m.SequenceNumber < active[keepIdx].SequenceNumber {
			keepIdx = i
		}
	}
	return active[keepIdx], nil, len(active) - 1
}

// nextSeedDelay returns the natural cadence plus a symmetric jitter in
// [-jitterBound, +jitterBound]. The jitter is operational (de-synchronising
// reseeds), not security-sensitive, so math/rand/v2 is the right tool.
func nextSeedDelay() time.Duration {
	// Int64N(2*jitterBound+1) yields [0, 2*jitterBound]; subtracting jitterBound
	// centres it on zero. The result is always strictly positive because
	// naturalCadence (5m) far exceeds jitterBound (10s). math/rand/v2 is correct
	// here: the jitter de-synchronises reseeds, it is not security-sensitive, so
	// crypto/rand would be needless ceremony (gosec G404 is a false positive).
	offset := time.Duration(mathrand.Int64N(int64(2*jitterBound)+1)) - jitterBound //nolint:gosec // non-security operational jitter
	return naturalCadence + offset
}
