using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Notifications;

internal sealed record ApnsJwtHeader(
    [property: JsonPropertyName("alg")] string Alg,
    [property: JsonPropertyName("kid")] string Kid);
