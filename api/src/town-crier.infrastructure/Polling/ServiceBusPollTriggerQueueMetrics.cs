using TownCrier.Application.Polling;
using TownCrier.Infrastructure.ServiceBus;

namespace TownCrier.Infrastructure.Polling;

/// <summary>
/// <see cref="IPollTriggerQueueMetrics"/> adapter over the Service Bus ARM
/// management-plane client. Reads <c>properties.countDetails.activeMessageCount</c>
/// and <c>scheduledMessageCount</c> via <see cref="IServiceBusManagementClient"/>.
/// Non-destructive; does not consume any messages. The ARM endpoint is required
/// — the data-plane REST surface returns Atom XML for the queue GET and lacks
/// <c>scheduledMessageCount</c> entirely (see ADR 0024 amendment 2026-04-23).
/// </summary>
public sealed class ServiceBusPollTriggerQueueMetrics : IPollTriggerQueueMetrics
{
    private readonly IServiceBusManagementClient managementClient;
    private readonly ServiceBusRestOptions options;

    internal ServiceBusPollTriggerQueueMetrics(
        IServiceBusManagementClient managementClient,
        ServiceBusRestOptions options)
    {
        ArgumentNullException.ThrowIfNull(managementClient);
        ArgumentNullException.ThrowIfNull(options);

        this.managementClient = managementClient;
        this.options = options;
    }

    public async Task<PollTriggerQueueDepth> GetDepthAsync(CancellationToken ct)
    {
        var counts = await this.managementClient
            .GetQueueDepthAsync(this.options.QueueName, ct)
            .ConfigureAwait(false);

        return new PollTriggerQueueDepth(
            ActiveMessageCount: counts.ActiveMessageCount,
            ScheduledMessageCount: counts.ScheduledMessageCount);
    }
}
