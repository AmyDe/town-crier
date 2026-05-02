using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Notifications;

/// <summary>
/// Body APNs returns on a non-2xx response, e.g. <c>{ "reason": "BadDeviceToken" }</c>.
/// Apple's published reason codes drive the sender's response handling.
/// </summary>
internal sealed record ApnsErrorResponse(
    [property: JsonPropertyName("reason")] string? Reason);
