using System.Diagnostics.CodeAnalysis;

namespace TownCrier.Application.Polling;

/// <summary>
/// Port over the Service Bus queue that drives the adaptive polling chain.
/// Under ADR 0024 amendment (2026-04-22) the queue uses
/// <b>receive-and-delete</b> mode — the message is destructively consumed on
/// receive, so there is no lock, no <c>Complete</c>, and no <c>Abandon</c>.
/// The orchestrator runs the handler first, then publishes the next trigger.
/// If anything fails between receive and publish the chain pauses until the
/// safety-net bootstrap recovers.
/// </summary>
[SuppressMessage(
    "Naming",
    "CA1711:Identifiers should not have incorrect suffix",
    Justification = "Queue reflects the underlying Service Bus abstraction.")]
public interface IPollTriggerQueue
{
    /// <summary>
    /// Destructively receives one message from the queue. Returns <c>null</c>
    /// when the queue is empty.
    /// </summary>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>The received message, or <c>null</c> if the queue is empty.</returns>
    Task<IPollTriggerMessage?> ReceiveAsync(CancellationToken ct);

    /// <summary>
    /// Publishes the next trigger message with <c>ScheduledEnqueueTimeUtc</c> set.
    /// </summary>
    /// <param name="scheduledEnqueueTime">When the next message should become visible to consumers.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>A task that completes when the message has been enqueued.</returns>
    Task PublishAtAsync(DateTimeOffset scheduledEnqueueTime, CancellationToken ct);
}
