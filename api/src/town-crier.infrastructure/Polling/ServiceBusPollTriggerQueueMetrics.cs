using TownCrier.Application.Polling;
using TownCrier.Infrastructure.ServiceBus;

namespace TownCrier.Infrastructure.Polling;

/// <summary>
/// <see cref="IPollTriggerQueueMetrics"/> adapter over the Service Bus
/// management API. Reads <c>countDetails.activeMessageCount</c> and
/// <c>countDetails.scheduledMessageCount</c> via
/// <see cref="IServiceBusRestClient.GetQueueDepthAsync"/>. Non-destructive;
/// see ADR 0024 amendment (2026-04-22) for the rationale — the previous
/// destructive PeekLock probe is replaced because receive-and-delete mode
/// would consume a live message on every tick, and the old probe was blind
/// to scheduled messages.
/// </summary>
public sealed class ServiceBusPollTriggerQueueMetrics : IPollTriggerQueueMetrics
{
    private readonly IServiceBusRestClient restClient;
    private readonly ServiceBusRestOptions options;

    internal ServiceBusPollTriggerQueueMetrics(
        IServiceBusRestClient restClient,
        ServiceBusRestOptions options)
    {
        ArgumentNullException.ThrowIfNull(restClient);
        ArgumentNullException.ThrowIfNull(options);

        this.restClient = restClient;
        this.options = options;
    }

    public async Task<PollTriggerQueueDepth> GetDepthAsync(CancellationToken ct)
    {
        var counts = await this.restClient
            .GetQueueDepthAsync(this.options.QueueName, ct)
            .ConfigureAwait(false);

        return new PollTriggerQueueDepth(
            ActiveMessageCount: counts.ActiveMessageCount,
            ScheduledMessageCount: counts.ScheduledMessageCount);
    }
}
