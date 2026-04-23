using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.ServiceBus;

/// <summary>
/// Shape of the ARM Service Bus GET-queue response. The management plane nests
/// <c>countDetails</c> under a top-level <c>properties</c> block; only the two
/// counts the bootstrapper needs are deserialised.
/// </summary>
internal sealed class QueueCountDetailsResponse
{
    [JsonPropertyName("properties")]
    public QueueProperties? Properties { get; init; }
}
