using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.ServiceBus;

internal sealed class BrokerProperties
{
    [JsonPropertyName("ScheduledEnqueueTimeUtc")]
    public string? ScheduledEnqueueTimeUtc { get; set; }
}
