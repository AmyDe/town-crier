using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Notifications;

internal sealed record ApnsDigestPayload(
    [property: JsonPropertyName("aps")] ApnsDigestAps Aps);
