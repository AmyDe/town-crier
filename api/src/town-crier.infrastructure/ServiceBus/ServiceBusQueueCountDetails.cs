namespace TownCrier.Infrastructure.ServiceBus;

internal readonly record struct ServiceBusQueueCountDetails(
    long ActiveMessageCount,
    long ScheduledMessageCount);
