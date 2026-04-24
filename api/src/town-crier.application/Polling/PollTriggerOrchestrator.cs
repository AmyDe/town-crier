using Microsoft.Extensions.Logging;

namespace TownCrier.Application.Polling;

/// <summary>
/// Single-run orchestrator that glues the Service Bus trigger queue to
/// <see cref="IPollPlanItCommandHandler"/>. Under the polling-lease-CAS spec
/// (docs/specs/polling-lease-cas.md) the ordering is
/// <b>acquire lease → receive → handler → publish → release</b>: the lease is
/// acquired before the destructive receive so no other actor can mutate the
/// queue during the critical section. One retry with a configurable delay
/// handles the common case where the bootstrap briefly holds the lease.
/// </summary>
public sealed partial class PollTriggerOrchestrator
{
    private readonly IPollPlanItCommandHandler handler;
    private readonly IPollTriggerQueue triggerQueue;
    private readonly PollNextRunScheduler scheduler;
    private readonly IPollingLeaseStore leaseStore;
    private readonly PollingOptions options;
    private readonly TimeProvider timeProvider;
    private readonly ILogger<PollTriggerOrchestrator> logger;

    public PollTriggerOrchestrator(
        IPollPlanItCommandHandler handler,
        IPollTriggerQueue triggerQueue,
        PollNextRunScheduler scheduler,
        IPollingLeaseStore leaseStore,
        PollingOptions options,
        TimeProvider timeProvider,
        ILogger<PollTriggerOrchestrator> logger)
    {
        this.handler = handler;
        this.triggerQueue = triggerQueue;
        this.scheduler = scheduler;
        this.leaseStore = leaseStore;
        this.options = options;
        this.timeProvider = timeProvider;
        this.logger = logger;
    }

    public async Task<PollTriggerOrchestratorRunResult> RunOnceAsync(CancellationToken ct)
    {
        var acquire = await this.leaseStore.TryAcquireAsync(this.options.OrchestratorLeaseTtl, ct).ConfigureAwait(false);
        if (!acquire.Acquired)
        {
            await Task.Delay(this.options.LeaseAcquireRetryDelay, ct).ConfigureAwait(false);
            acquire = await this.leaseStore.TryAcquireAsync(this.options.OrchestratorLeaseTtl, ct).ConfigureAwait(false);
            if (!acquire.Acquired)
            {
                LogLeaseUnavailable(this.logger);
                return new PollTriggerOrchestratorRunResult(
                    MessageReceived: false,
                    PublishedNext: false,
                    PollResult: null,
                    LeaseUnavailable: true);
            }
        }

        try
        {
            var message = await this.triggerQueue.ReceiveAsync(ct).ConfigureAwait(false);
            if (message is null)
            {
                LogEmptyQueue(this.logger);
                return new PollTriggerOrchestratorRunResult(
                    MessageReceived: false,
                    PublishedNext: false,
                    PollResult: null,
                    LeaseUnavailable: false);
            }

            var pollResult = await this.handler.HandleAsync(new PollPlanItCommand(), ct).ConfigureAwait(false);

            var now = this.timeProvider.GetUtcNow();
            var nextRun = this.scheduler.ComputeNextRun(pollResult.TerminationReason, pollResult.RetryAfter, now);

            await this.triggerQueue.PublishAtAsync(nextRun, ct).ConfigureAwait(false);

            return new PollTriggerOrchestratorRunResult(
                MessageReceived: true,
                PublishedNext: true,
                PollResult: pollResult,
                LeaseUnavailable: false);
        }
        finally
        {
            await this.leaseStore.ReleaseAsync(acquire.Handle!, ct).ConfigureAwait(false);
        }
    }

    [LoggerMessage(Level = LogLevel.Information, Message = "Poll trigger queue empty, exiting cleanly (bootstrap will re-seed)")]
    private static partial void LogEmptyQueue(ILogger logger);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Polling lease unavailable after retry — exiting; KEDA will re-trigger when peer releases")]
    private static partial void LogLeaseUnavailable(ILogger logger);
}
