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

    Task<ReceivedServiceBusMessage?> ReceiveOneAsync(
        string queueName,
        TimeSpan timeout,
        CancellationToken ct);

    Task CompleteAsync(Uri lockUrl, CancellationToken ct);

    Task AbandonAsync(Uri lockUrl, CancellationToken ct);
}

internal sealed class ReceivedServiceBusMessage
{
    public required byte[] Body { get; init; }

    public required Uri LockUrl { get; init; }
}
