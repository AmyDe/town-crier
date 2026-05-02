using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Notifications;

internal sealed record ApnsJwtPayload(
    [property: JsonPropertyName("iss")] string Iss,
    [property: JsonPropertyName("iat")] long Iat);
