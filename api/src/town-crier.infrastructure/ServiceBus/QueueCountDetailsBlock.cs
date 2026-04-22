using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.ServiceBus;

internal sealed class QueueCountDetailsBlock
{
    [JsonPropertyName("activeMessageCount")]
    public long ActiveMessageCount { get; init; }

    [JsonPropertyName("scheduledMessageCount")]
    public long ScheduledMessageCount { get; init; }
}
