using Microsoft.Extensions.Logging;

namespace TownCrier.Application.Polling;

/// <summary>
/// Safety-net re-seed for the Service-Bus-coordinated polling chain. Runs after
/// the cron-driven <see cref="PollPlanItCommandHandler"/> call in the worker's
/// safety-net branch — if the trigger queue is empty (because the SB cycle has
/// never bootstrapped, or has died), publish a single jittered bootstrap
/// trigger so the adaptive cycle self-heals.
///
/// <para>
/// Implementation uses <see cref="IPollTriggerQueue.ReceiveAsync"/> with the
/// existing PeekLock semantics. If the receive returns <c>null</c> → queue is
/// empty → publish. If it returns a message → abandon the lock (so the real
/// poll-sb consumer redelivers and processes it) and skip publishing. This has
/// a small TOCTOU window but is acceptable for a best-effort seed — the
/// orchestrator is idempotent under the Cosmos lease guard and duplicate seeds
/// are harmless.
/// </para>
///
/// <para>
/// All failures in the probe OR the publish are swallowed: the safety-net's
/// primary job is to poll PlanIt, and a failed reseed is retried on the next
/// cron tick. The caller's exit code must reflect the poll outcome, not the
/// reseed outcome.
/// </para>
/// </summary>
public sealed partial class PollTriggerBootstrapper
{
    private readonly IPollTriggerQueue triggerQueue;
    private readonly PollNextRunScheduler scheduler;
    private readonly TimeProvider timeProvider;
    private readonly ILogger<PollTriggerBootstrapper> logger;

    public PollTriggerBootstrapper(
        IPollTriggerQueue triggerQueue,
        PollNextRunScheduler scheduler,
        TimeProvider timeProvider,
        ILogger<PollTriggerBootstrapper> logger)
    {
        ArgumentNullException.ThrowIfNull(triggerQueue);
        ArgumentNullException.ThrowIfNull(scheduler);
        ArgumentNullException.ThrowIfNull(timeProvider);
        ArgumentNullException.ThrowIfNull(logger);

        this.triggerQueue = triggerQueue;
        this.scheduler = scheduler;
        this.timeProvider = timeProvider;
        this.logger = logger;
    }

    public async Task<PollTriggerBootstrapResult> TryBootstrapAsync(CancellationToken ct)
    {
        IPollTriggerMessage? existing;
        try
        {
            existing = await this.triggerQueue.ReceiveAsync(ct).ConfigureAwait(false);
        }
#pragma warning disable CA1031 // Best-effort reseed — any probe failure is absorbed.
        catch (Exception ex)
#pragma warning restore CA1031
        {
            LogProbeFailed(this.logger, ex);
            return new PollTriggerBootstrapResult(Published: false, ProbeFailed: true);
        }

        if (existing is not null)
        {
            // Queue is non-empty — the poll-sb cycle is alive. Abandon the
            // PeekLock so the real consumer redelivers and settles the message.
            try
            {
                await this.triggerQueue.AbandonAsync(existing, ct).ConfigureAwait(false);
            }
#pragma warning disable CA1031 // Abandon is best-effort; the lock will expire naturally.
            catch (Exception ex)
#pragma warning restore CA1031
            {
                LogAbandonFailed(this.logger, ex);
            }

            LogQueueAlreadySeeded(this.logger);
            return new PollTriggerBootstrapResult(Published: false, ProbeFailed: false);
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
            return new PollTriggerBootstrapResult(Published: false, ProbeFailed: true);
        }

        LogBootstrapPublished(this.logger, nextRun);
        return new PollTriggerBootstrapResult(Published: true, ProbeFailed: false);
    }

    [LoggerMessage(Level = LogLevel.Warning, Message = "Safety-net queue probe failed; skipping reseed (handler already ran)")]
    private static partial void LogProbeFailed(ILogger logger, Exception ex);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Safety-net queue abandon failed; PeekLock will expire naturally")]
    private static partial void LogAbandonFailed(ILogger logger, Exception ex);

    [LoggerMessage(Level = LogLevel.Warning, Message = "Safety-net bootstrap publish failed; next cron tick will retry")]
    private static partial void LogPublishFailed(ILogger logger, Exception ex);

    [LoggerMessage(Level = LogLevel.Information, Message = "Safety-net skipped reseed — poll trigger queue already has a pending message")]
    private static partial void LogQueueAlreadySeeded(ILogger logger);

    [LoggerMessage(Level = LogLevel.Information, Message = "Safety-net published bootstrap poll trigger scheduled for {ScheduledEnqueueTimeUtc:o}")]
    private static partial void LogBootstrapPublished(ILogger logger, DateTimeOffset scheduledEnqueueTimeUtc);
}
