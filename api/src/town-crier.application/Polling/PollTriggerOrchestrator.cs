using Microsoft.Extensions.Logging;

namespace TownCrier.Application.Polling;

/// <summary>
/// Single-run orchestrator that glues the Service Bus trigger queue to
/// <see cref="PollPlanItCommandHandler"/>. Under ADR 0024 amendment
/// (2026-04-22) the queue is consumed in receive-and-delete mode and the
/// ordering is <b>receive → handler → publish</b>: the trigger message is
/// destructively consumed on receive, the handler runs, then the next
/// trigger is published with a scheduled enqueue time. If anything fails
/// between receive and publish the chain pauses until the safety-net
/// bootstrap (<see cref="PollTriggerBootstrapper"/>) recovers it on its
/// cron tick.
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

        // Message is destructively consumed — handler runs first. If it throws
        // we let the exception bubble up; the safety-net bootstrap recovers
        // the chain on its next cron tick.
        var pollResult = await this.handler.HandleAsync(new PollPlanItCommand(), ct).ConfigureAwait(false);

        if (pollResult.TerminationReason == PollTerminationReason.LeaseHeld)
        {
            // Another replica holds the Cosmos lease and is responsible for
            // publishing the next trigger. Our message is already gone — exit
            // without publishing so we don't duplicate the next trigger.
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

        // Consume-before-publish — destructive receive already happened, so
        // this is the sole outstanding write that could leave the chain in a
        // quiescent state. A publish failure surfaces to the caller; the
        // safety-net recovers.
        await this.triggerQueue.PublishAtAsync(nextRun, ct).ConfigureAwait(false);

        return new PollTriggerOrchestratorRunResult(
            MessageReceived: true,
            PublishedNext: true,
            PollResult: pollResult);
    }

    [LoggerMessage(Level = LogLevel.Information, Message = "Poll trigger queue empty, exiting cleanly (safety-net run will re-seed)")]
    private static partial void LogEmptyQueue(ILogger logger);
}
