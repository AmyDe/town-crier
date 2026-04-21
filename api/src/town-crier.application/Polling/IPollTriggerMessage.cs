namespace TownCrier.Application.Polling;

/// <summary>
/// Opaque handle for a received Service Bus trigger message. The infrastructure
/// adapter carries whatever it needs (ServiceBusReceivedMessage, receiver, etc.)
/// to complete or abandon the message later in the cycle.
/// </summary>
public interface IPollTriggerMessage
{
    string Id { get; }
}
