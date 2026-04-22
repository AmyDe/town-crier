using System.Diagnostics.CodeAnalysis;
using System.Globalization;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.ServiceBus;

namespace TownCrier.Infrastructure.Polling;

/// <summary>
/// <see cref="IPollTriggerQueue"/> adapter over the Service Bus REST client.
/// Under ADR 0024 amendment (2026-04-22) the adapter uses receive-and-delete
/// mode — the message is destructively consumed on receive, so there is no
/// lock URL to thread through, no <c>Complete</c>, and no <c>Abandon</c>.
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

        return new ServiceBusPollTriggerMessage();
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

    private sealed class ServiceBusPollTriggerMessage : IPollTriggerMessage
    {
        public string Id { get; } = Guid.NewGuid().ToString("N");
    }
}
