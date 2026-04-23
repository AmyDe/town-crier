using System.Text.Json.Serialization.Metadata;

namespace TownCrier.Infrastructure.ServiceBus;

internal interface IServiceBusRestClient
{
    Task PublishAsync<T>(
        string queueName,
        T payload,
        DateTimeOffset? scheduledEnqueueTimeUtc,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct);

    /// <summary>
    /// Destructively receives one message from the queue via receive-and-delete
    /// mode. Returns <c>null</c> when the queue is empty.
    /// </summary>
    /// <param name="queueName">Queue to receive from.</param>
    /// <param name="timeout">Server-side long-poll timeout.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>The received message, or <c>null</c> if the queue is empty.</returns>
    Task<ReceivedServiceBusMessage?> ReceiveOneAsync(
        string queueName,
        TimeSpan timeout,
        CancellationToken ct);
}
