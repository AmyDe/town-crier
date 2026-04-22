namespace TownCrier.Infrastructure.ServiceBus;

internal sealed class ReceivedServiceBusMessage
{
    public required byte[] Body { get; init; }
}
