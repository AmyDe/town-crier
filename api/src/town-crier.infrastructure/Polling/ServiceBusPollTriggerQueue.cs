using System.Diagnostics.CodeAnalysis;
using System.Globalization;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.ServiceBus;

namespace TownCrier.Infrastructure.Polling;

/// <summary>
/// <see cref="IPollTriggerQueue"/> adapter over the Service Bus REST client.
/// Carries the PeekLock URL on the received message so <see cref="CompleteAsync"/>
/// and <see cref="AbandonAsync"/> can settle the right message without a
/// round-trip to re-fetch broker state.
/// </summary>
[SuppressMessage(
    "Naming",
    "CA1711:Identifiers should not have incorrect suffix",
    Justification = "Queue reflects the underlying Service Bus abstraction.")]
public sealed class ServiceBusPollTriggerQueue : IPollTriggerQueue
{
    private readonly IServiceBusRestClient restClient;
    private readonly ServiceBusRestOptions options;

    internal ServiceBusPollTriggerQueue(
        IServiceBusRestClient restClient,
        ServiceBusRestOptions options)
    {
        ArgumentNullException.ThrowIfNull(restClient);
        ArgumentNullException.ThrowIfNull(options);

        this.restClient = restClient;
        this.options = options;
    }

    public async Task<IPollTriggerMessage?> ReceiveAsync(CancellationToken ct)
    {
        var received = await this.restClient.ReceiveOneAsync(
            this.options.QueueName,
            TimeSpan.FromSeconds(5),
            ct).ConfigureAwait(false);

        if (received is null)
        {
            return null;
        }

        return new ServiceBusPollTriggerMessage(received.LockUrl);
    }

    public async Task PublishAtAsync(DateTimeOffset scheduledEnqueueTime, CancellationToken ct)
    {
        var payload = new PollTriggerPayload
        {
            PublishedAtUtc = DateTimeOffset.UtcNow.ToString("o", CultureInfo.InvariantCulture),
        };

        await this.restClient.PublishAsync(
            this.options.QueueName,
            payload,
            scheduledEnqueueTime,
            PollingJsonSerializerContext.Default.PollTriggerPayload,
            ct).ConfigureAwait(false);
    }

    public async Task CompleteAsync(IPollTriggerMessage message, CancellationToken ct)
    {
        var lockUrl = RequireLockUrl(message);
        await this.restClient.CompleteAsync(lockUrl, ct).ConfigureAwait(false);
    }

    public async Task AbandonAsync(IPollTriggerMessage message, CancellationToken ct)
    {
        var lockUrl = RequireLockUrl(message);
        await this.restClient.AbandonAsync(lockUrl, ct).ConfigureAwait(false);
    }

    private static Uri RequireLockUrl(IPollTriggerMessage message)
    {
        ArgumentNullException.ThrowIfNull(message);

        if (message is not ServiceBusPollTriggerMessage sbMessage)
        {
            throw new ArgumentException(
                $"Message must be produced by {nameof(ServiceBusPollTriggerQueue)}.{nameof(ReceiveAsync)}.",
                nameof(message));
        }

        return sbMessage.LockUrl;
    }

    private sealed class ServiceBusPollTriggerMessage : IPollTriggerMessage
    {
        public ServiceBusPollTriggerMessage(Uri lockUrl)
        {
            this.LockUrl = lockUrl;
            this.Id = lockUrl.ToString();
        }

        public string Id { get; }

        public Uri LockUrl { get; }
    }
}
