using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Auth;

internal sealed record Auth0TokenRequest(
    [property: JsonPropertyName("grant_type")] string GrantType,
    [property: JsonPropertyName("client_id")] string ClientId,
    [property: JsonPropertyName("client_secret")] string ClientSecret,
    [property: JsonPropertyName("audience")] string Audience);
