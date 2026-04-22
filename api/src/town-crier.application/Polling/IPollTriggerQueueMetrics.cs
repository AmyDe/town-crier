namespace TownCrier.Application.Polling;

/// <summary>
/// Port over the Service Bus management API that reports queue depth counters
/// used by <see cref="PollTriggerBootstrapper"/> to decide whether the adaptive
/// polling chain is alive. Replaces the destructive <c>ReceiveAsync</c> probe
/// that was incompatible with the receive-and-delete mode introduced by
/// ADR 0024 and blind to scheduled (future-dated) messages.
/// </summary>
public interface IPollTriggerQueueMetrics
{
    /// <summary>
    /// Reads the queue's current active and scheduled message counts via the
    /// Service Bus management API (<c>countDetails.activeMessageCount</c> and
    /// <c>countDetails.scheduledMessageCount</c>). The bootstrapper seeds the
    /// chain only when both values are zero.
    /// </summary>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>The current <see cref="PollTriggerQueueDepth"/>.</returns>
    Task<PollTriggerQueueDepth> GetDepthAsync(CancellationToken ct);
}

/// <summary>
/// Snapshot of the poll trigger queue's active and scheduled message counts.
/// </summary>
/// <param name="ActiveMessageCount">Messages currently available for consumption.</param>
/// <param name="ScheduledMessageCount">Messages with <c>ScheduledEnqueueTimeUtc</c> in the future.</param>
public readonly record struct PollTriggerQueueDepth(long ActiveMessageCount, long ScheduledMessageCount)
{
    /// <summary>
    /// Gets a value indicating whether the queue has no active or scheduled messages
    /// — the signal the bootstrapper uses to seed a new trigger.
    /// </summary>
    public bool IsEmpty => this.ActiveMessageCount == 0 && this.ScheduledMessageCount == 0;
}
