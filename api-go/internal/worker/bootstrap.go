// Package worker holds the Town Crier background-worker modes that run as
// short-lived Container Apps Jobs. The poll-bootstrap mode is the tracer-bullet
// slice: a safety net that reseeds the poll-trigger queue when the adaptive
// polling chain has gone silent. It shares the Postgres polling lease with the
// orchestrator (poll-sb mode): only the current lease holder may mutate the
// trigger queue, so a bootstrap tick landing mid-cycle cannot fork the chain
// (GH#938 PR1). The heavier poll-sb / digest / dormant-cleanup modes land in
// later beads (see epic tc-wad3).
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

// naturalCadence and jitterBound define the reseed schedule: a 5-minute base
// delay with up to 10s of jitter on either side. The bootstrap trigger represents
// a healthy cycle starting from scratch, so it uses the same cadence the
// orchestrator would.
const (
	naturalCadence = 5 * time.Minute
	jitterBound    = 10 * time.Second

	// bootstrapLeaseTTL is the polling-lease TTL the bootstrap requests. A probe
	// + publish is a sub-second operation, so a short TTL suffices; it never
	// holds the lease anywhere near the natural-cadence gap between reseeds
	// (GH#938 PR1).
	bootstrapLeaseTTL = 1 * time.Minute
)

// triggerQueue is the consumer-side interface the bootstrapper depends on. It is
// satisfied by *servicebus.Client in production and by a hand-written fake in
// tests. Only the two methods the bootstrapper uses are declared here.
type triggerQueue interface {
	QueueDepth(ctx context.Context) (servicebus.QueueDepth, error)
	PublishAt(ctx context.Context, scheduledEnqueueTime time.Time, body []byte) error
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

// BootstrapResult is the outcome of TryBootstrap. The field names match the
// App Insights telemetry tags (polling.safety_net.bootstrap_published /
// bootstrap_probe_failed).
type BootstrapResult struct {
	// Published reports whether a seed trigger was successfully published.
	Published bool
	// ProbeFailed reports whether the depth probe OR the publish failed. Both are
	// absorbed (a failed reseed retries on the next cron tick); the flag exists
	// purely for telemetry.
	ProbeFailed bool
	// LeaseUnavailable reports that the polling lease was held by a peer (the
	// chain owner is alive), so TryBootstrap skipped cleanly without probing or
	// publishing. Mirrors OrchestratorRunResult.LeaseUnavailable.
	LeaseUnavailable bool
}

// Bootstrapper is the poll-trigger safety net. Under the shared Postgres polling
// lease, it probes the trigger queue and, only when the queue is completely
// empty, publishes one jittered seed trigger scheduled for the future. It never
// invokes the poll handler — that is the orchestrator's job (poll-sb mode).
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

// TryBootstrap acquires the polling lease, then probes the queue and reseeds it
// if and only if it is empty, releasing the lease afterwards. When the lease is
// held by a peer, TryBootstrap is a clean no-op: the chain owner is alive, and if
// it had instead crashed, the lease's own TTL expiry lets the next cron tick
// reseed (Pre-Resolved Design Decision, GH#938 — bootstrap skips rather than
// waits/retries). All lease-acquire, probe and publish failures are absorbed
// into the returned result (never returned as an error) so a transient failure
// does not fail the job — the next cron tick retries. The returned error is
// reserved for caller-side concerns; today it is always nil.
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
		// Held by a peer: the chain owner is alive, so this is a clean no-op, not a
		// probe failure.
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

	if !depth.IsEmpty() {
		b.logger.InfoContext(ctx, "poll-bootstrap skipped; trigger queue already seeded",
			"active", depth.ActiveMessageCount,
			"scheduled", depth.ScheduledMessageCount)
		return BootstrapResult{}, nil
	}

	now := b.now()
	scheduledAt := now.Add(nextSeedDelay())

	body, err := json.Marshal(triggerPayload{PublishedAtUtc: now.UTC().Format(time.RFC3339Nano)})
	if err != nil {
		// json.Marshal of a fixed struct cannot realistically fail; treat any
		// failure as a probe failure for telemetry parity rather than panicking.
		b.logger.WarnContext(ctx, "poll-bootstrap payload marshal failed; skipping reseed", "error", err)
		return BootstrapResult{ProbeFailed: true}, nil
	}

	if err := b.queue.PublishAt(ctx, scheduledAt, body); err != nil {
		b.logger.WarnContext(ctx, "poll-bootstrap publish failed; next cron tick will retry", "error", err)
		return BootstrapResult{ProbeFailed: true}, nil
	}

	b.logger.InfoContext(ctx, "poll-bootstrap published seed trigger", "scheduledAt", scheduledAt.UTC())
	return BootstrapResult{Published: true}, nil
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
