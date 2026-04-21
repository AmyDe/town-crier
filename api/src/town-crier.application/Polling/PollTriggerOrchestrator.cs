using Microsoft.Extensions.Logging;

namespace TownCrier.Application.Polling;

/// <summary>
/// Single-run orchestrator that glues the Service Bus trigger queue to
/// <see cref="PollPlanItCommandHandler"/>. Responsible for the load-bearing
/// publish-before-ack ordering: publish the next trigger first, then complete
/// the one that drove this run. If the worker crashes between the publish and
/// the ack, the trigger redelivers via PeekLock and the chain recovers.
/// </summary>
public sealed partial class PollTriggerOrchestrator
{
    private readonly PollPlanItCommandHandler handler;
    private readonly IPollTriggerQueue triggerQueue;
    private readonly PollNextRunScheduler scheduler;
    private readonly TimeProvider timeProvider;
    private readonly ILogger<PollTriggerOrchestrator> logger;

    public PollTriggerOrchestrator(
        PollPlanItCommandHandler handler,
        IPollTriggerQueue triggerQueue,
        PollNextRunScheduler scheduler,
        TimeProvider timeProvider,
        ILogger<PollTriggerOrchestrator> logger)
    {
        this.handler = handler;
        this.triggerQueue = triggerQueue;
        this.scheduler = scheduler;
        this.timeProvider = timeProvider;
        this.logger = logger;
    }

    public async Task<PollTriggerOrchestratorRunResult> RunOnceAsync(CancellationToken ct)
    {
        var message = await this.triggerQueue.ReceiveAsync(ct).ConfigureAwait(false);
        if (message is null)
        {
            LogEmptyQueue(this.logger);
            return new PollTriggerOrchestratorRunResult(
                MessageReceived: false,
                PublishedNext: false,
                PollResult: null);
        }

        PollPlanItResult pollResult;
        try
        {
            pollResult = await this.handler.HandleAsync(new PollPlanItCommand(), ct).ConfigureAwait(false);
        }
        catch
        {
            await this.triggerQueue.AbandonAsync(message, ct).ConfigureAwait(false);
            throw;
        }

        if (pollResult.TerminationReason == PollTerminationReason.LeaseHeld)
        {
            // Lease holder is responsible for publishing the next trigger.
            // Abandon the PeekLock so the message redelivers after the lock
            // expires — if the holder crashed without publishing, this run
            // (or the next replica) will take over.
            await this.triggerQueue.AbandonAsync(message, ct).ConfigureAwait(false);
            return new PollTriggerOrchestratorRunResult(
                MessageReceived: true,
                PublishedNext: false,
                PollResult: pollResult);
        }

        var now = this.timeProvider.GetUtcNow();
        var nextRun = this.scheduler.ComputeNextRun(
            pollResult.TerminationReason,
            pollResult.RetryAfter,
            now);

        // Publish BEFORE complete — load-bearing for crash safety.
        await this.triggerQueue.PublishAtAsync(nextRun, ct).ConfigureAwait(false);
        await this.triggerQueue.CompleteAsync(message, ct).ConfigureAwait(false);

        return new PollTriggerOrchestratorRunResult(
            MessageReceived: true,
            PublishedNext: true,
            PollResult: pollResult);
    }

    [LoggerMessage(Level = LogLevel.Information, Message = "Poll trigger queue empty, exiting cleanly (safety-net run will re-seed)")]
    private static partial void LogEmptyQueue(ILogger logger);
}
