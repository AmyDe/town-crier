using System.Diagnostics.CodeAnalysis;

namespace TownCrier.Application.Polling;

/// <summary>
/// Port over the Service Bus queue that drives the adaptive polling chain. The
/// worker receives a single trigger message per run, polls PlanIt, publishes the
/// next trigger (with a scheduled enqueue time), then completes the original.
/// The publish-before-ack ordering is load-bearing: if the worker crashes between
/// publish and ack, the original message redelivers via PeekLock and the chain
/// recovers without a safety-net bootstrap.
/// </summary>
[SuppressMessage(
    "Naming",
    "CA1711:Identifiers should not have incorrect suffix",
    Justification = "Queue reflects the underlying Service Bus abstraction.")]
public interface IPollTriggerQueue
{
    /// <summary>
    /// Receives one message from the queue in PeekLock mode. Returns <c>null</c>
    /// when the queue is empty (safety-net runs experience this when the Service
    /// Bus chain is alive).
    /// </summary>
    Task<IPollTriggerMessage?> ReceiveAsync(CancellationToken ct);

    /// <summary>
    /// Publishes the next trigger message with <c>ScheduledEnqueueTimeUtc</c> set.
    /// </summary>
    Task PublishAtAsync(DateTimeOffset scheduledEnqueueTime, CancellationToken ct);

    /// <summary>
    /// Completes (acks) the message, removing it from the queue.
    /// </summary>
    Task CompleteAsync(IPollTriggerMessage message, CancellationToken ct);

    /// <summary>
    /// Abandons the PeekLock, returning the message to the queue for redelivery.
    /// Used when the current run cannot acquire the lease — the holder is
    /// expected to publish the next trigger, but we release this lock so the
    /// holder (if it later frees the lease without publishing) or another
    /// replica gets a chance to pick it up.
    /// </summary>
    Task AbandonAsync(IPollTriggerMessage message, CancellationToken ct);
}
