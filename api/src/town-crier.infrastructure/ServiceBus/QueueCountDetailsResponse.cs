using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.ServiceBus;

/// <summary>
/// Shape of the Service Bus management-API GET-queue response used to read
/// <c>countDetails.activeMessageCount + scheduledMessageCount</c>. Only the
/// fields the bootstrapper needs are deserialised.
/// </summary>
internal sealed class QueueCountDetailsResponse
{
    [JsonPropertyName("countDetails")]
    public QueueCountDetailsBlock? CountDetails { get; init; }
}
