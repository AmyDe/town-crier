using Microsoft.Extensions.Logging;
using TownCrier.Application.Observability;

namespace TownCrier.Application.Polling;

/// <summary>
/// Safety-net re-seed for the Service-Bus-coordinated polling chain. Runs on
/// a cron tick as the sole recovery mechanism for a silent chain (e.g. the
/// handler crashed after receiving a trigger but before publishing the next
/// one, Service Bus maintenance dropped the chain, the queue was manually
/// purged, etc.).
///
/// <para>
/// Under ADR 0024 amendment (2026-04-22) the bootstrapper first acquires the
/// polling lease (via <see cref="IPollingLeaseStore"/>) before touching the
/// queue. If the orchestrator currently holds the lease the bootstrap exits
/// immediately — the chain is alive by definition. The lease is released in a
/// <c>finally</c> block regardless of outcome.
/// </para>
///
/// <para>
/// With the lease held the bootstrapper probes the queue via the Service Bus
/// management API (<see cref="IPollTriggerQueueMetrics"/>) reading
/// <c>countDetails.activeMessageCount + scheduledMessageCount</c>.
/// A bootstrap seed is published only when both counts are zero. The
/// previous destructive PeekLock probe is replaced because (a) it would
/// delete a live message every 30 min under receive-and-delete mode and
/// (b) it was blind to scheduled (future-dated) messages, occasionally
/// double-publishing when a healthy chain was paused on Retry-After.
/// </para>
///
/// <para>
/// All failures in the probe OR the publish are swallowed: the safety-net's
/// primary job is to poll PlanIt, and a failed reseed is retried on the next
/// cron tick.
/// </para>
/// </summary>
public sealed partial class PollTriggerBootstrapper
{
    private readonly IPollTriggerQueue triggerQueue;
    private readonly IPollTriggerQueueMetrics metrics;
    private readonly PollNextRunScheduler scheduler;
    private readonly IPollingLeaseStore leaseStore;
    private readonly PollingOptions options;
    private readonly TimeProvider timeProvider;
    private readonly ILogger<PollTriggerBootstrapper> logger;

    public PollTriggerBootstrapper(
        IPollTriggerQueue triggerQueue,
        IPollTriggerQueueMetrics metrics,
        PollNextRunScheduler scheduler,
        IPollingLeaseStore leaseStore,
        PollingOptions options,
        TimeProvider timeProvider,
        ILogger<PollTriggerBootstrapper> logger)
    {
        ArgumentNullException.ThrowIfNull(triggerQueue);
        ArgumentNullException.ThrowIfNull(metrics);
        ArgumentNullException.ThrowIfNull(scheduler);
        ArgumentNullException.ThrowIfNull(leaseStore);
        ArgumentNullException.ThrowIfNull(options);
        ArgumentNullException.ThrowIfNull(timeProvider);
        ArgumentNullException.ThrowIfNull(logger);

        this.triggerQueue = triggerQueue;
        this.metrics = metrics;
        this.scheduler = scheduler;
        this.leaseStore = leaseStore;
        this.options = options;
        this.timeProvider = timeProvider;
        this.logger = logger;
    }

    public async Task<PollTriggerBootstrapResult> TryBootstrapAsync(CancellationToken ct)
    {
        var callerTag = new KeyValuePair<string, object?>("caller", "bootstrap");

        var acquire = await this.leaseStore.TryAcquireAsync(this.options.BootstrapLeaseTtl, ct).ConfigureAwait(false);
        if (!acquire.Acquired)
        {
            LogLeaseHeldByPeer(this.logger);
            PollingMetrics.LeaseHeldByPeer.Add(1, callerTag);
            return new PollTriggerBootstrapResult(Published: false, ProbeFailed: false, LeaseUnavailable: true);
        }

        PollingMetrics.LeaseAcquired.Add(1, callerTag);

        try
        {
            PollTriggerQueueDepth depth;
            try
            {
                depth = await this.metrics.GetDepthAsync(ct).ConfigureAwait(false);
            }
#pragma warning disable CA1031 // Best-effort reseed — any probe failure is absorbed.
            catch (Exception ex)
#pragma warning restore CA1031
            {
                LogProbeFailed(this.logger, ex);
                return new PollTriggerBootstrapResult(Published: false, ProbeFailed: true, LeaseUnavailable: false);
            }

            if (!depth.IsEmpty)
            {
                LogQueueAlreadySeeded(
                    this.logger,
                    depth.ActiveMessageCount,
                    depth.ScheduledMessageCount);
                return new PollTriggerBootstrapResult(Published: false, ProbeFailed: false, LeaseUnavailable: false);
            }

            var now = this.timeProvider.GetUtcNow();

            // Use the Natural cadence scheduling path — the bootstrap trigger
            // represents a healthy cycle starting from scratch, with normal
            // jitter applied by the scheduler's IPollJitter.
            var nextRun = this.scheduler.ComputeNextRun(
                PollTerminationReason.Natural,
                retryAfter: null,
                now);

            try
            {
                await this.triggerQueue.PublishAtAsync(nextRun, ct).ConfigureAwait(false);
            }
#pragma warning disable CA1031 // Best-effort reseed — any publish failure is absorbed.
            catch (Exception ex)
#pragma warning restore CA1031
            {
                LogPublishFailed(this.logger, ex);
                return new PollTriggerBootstrapResult(Published: false, ProbeFailed: true, LeaseUnavailable: false);
            }

            LogBootstrapPublished(this.logger, nextRun);
            return new PollTriggerBootstrapResult(Published: true, ProbeFailed: false, LeaseUnavailable: false);
        }
        finally
        {
            var releaseOutcome = await this.leaseStore.ReleaseAsync(acquire.Handle!, ct).ConfigureAwait(false);
            if (releaseOutcome == LeaseReleaseOutcome.PreconditionFailed)
            {
                LogRelease412(this.logger);
                PollingMetrics.LeaseReleased412.Add(1, callerTag);
            }
        }
    }

    [LoggerMessage(Level = LogLevel.Information, Message = "Safety-net bootstrap skipped — peer holds the polling lease (orchestrator is running)")]
    private static partial void LogLeaseHeldByPeer(ILogger logger);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Safety-net queue metrics probe failed; skipping reseed (handler already ran)")]
    private static partial void LogProbeFailed(ILogger logger, Exception ex);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Safety-net bootstrap publish failed; next cron tick will retry")]
    private static partial void LogPublishFailed(ILogger logger, Exception ex);

    [LoggerMessage(Level = LogLevel.Information, Message = "Safety-net skipped reseed — poll trigger queue already has {ActiveMessageCount} active and {ScheduledMessageCount} scheduled messages")]
    private static partial void LogQueueAlreadySeeded(ILogger logger, long activeMessageCount, long scheduledMessageCount);

    [LoggerMessage(Level = LogLevel.Information, Message = "Safety-net published bootstrap poll trigger scheduled for {ScheduledEnqueueTimeUtc:o}")]
    private static partial void LogBootstrapPublished(ILogger logger, DateTimeOffset scheduledEnqueueTimeUtc);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Safety-net lease release returned 412 PreconditionFailed — ETag mismatch; TTL is the backstop")]
    private static partial void LogRelease412(ILogger logger);
}
