using System.Text.Json.Serialization;

namespace TownCrier.Infrastructure.Auth;

internal sealed record Auth0TokenResponse(
    [property: JsonPropertyName("access_token")] string AccessToken,
    [property: JsonPropertyName("expires_in")] int ExpiresIn);
