namespace TownCrier.Infrastructure.ServiceBus;

public sealed class ServiceBusRestOptions
{
    public required string Namespace { get; init; }

    public required string QueueName { get; init; }
}
