namespace TownCrier.Infrastructure.ServiceBus;

/// <summary>
/// ARM management-plane client for Service Bus queue metadata. Separate from
/// <see cref="IServiceBusRestClient"/> because the management plane has a
/// different endpoint (management.azure.com), path shape
/// (/subscriptions/.../queues/{name}), api-version (2021-11-01), and token
/// audience (https://management.azure.com/.default).
/// </summary>
internal interface IServiceBusManagementClient
{
    /// <summary>
    /// Reads the queue's active and scheduled message counts from the ARM
    /// management plane. Non-destructive; does not consume any messages. The
    /// caller's managed identity must have Reader (or richer) on the queue
    /// resource or its namespace.
    /// </summary>
    /// <param name="queueName">Queue whose counts are read.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>Active and scheduled message counts for the queue.</returns>
    Task<ServiceBusQueueCountDetails> GetQueueDepthAsync(string queueName, CancellationToken ct);
}
