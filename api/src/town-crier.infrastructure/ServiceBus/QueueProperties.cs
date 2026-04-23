using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.ServiceBus;

/// <summary>
/// The <c>properties</c> block of the ARM Service Bus GET-queue response. Only
/// <c>countDetails</c> is surfaced — the bootstrapper has no need for the
/// configuration fields (lockDuration, maxDeliveryCount, etc.).
/// </summary>
internal sealed class QueueProperties
{
    [JsonPropertyName("countDetails")]
    public QueueCountDetailsBlock? CountDetails { get; init; }
}
