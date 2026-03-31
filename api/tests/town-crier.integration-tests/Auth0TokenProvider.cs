using System.Net.Http.Json;
using System.Text.Json.Serialization;

namespace TownCrier.IntegrationTests;

internal static class Auth0TokenProvider
{
    private static readonly Lazy<Task<string>> TokenTask = new(
        () => AcquireTokenAsync(new HttpClient()));

    public static Task<string> GetTokenAsync() => TokenTask.Value;

    internal static async Task<string> AcquireTokenAsync(HttpClient client)
    {
        var tokenUrl = $"https://{IntegrationTestConfig.Auth0Domain}/oauth/token";

        var payload = new Dictionary<string, string>
        {
            ["grant_type"] = "password",
            ["client_id"] = IntegrationTestConfig.Auth0ClientId,
            ["username"] = IntegrationTestConfig.Username,
            ["password"] = IntegrationTestConfig.Password,
            ["audience"] = IntegrationTestConfig.Auth0Audience,
            ["scope"] = "openid",
        };

        if (IntegrationTestConfig.Auth0ClientSecret is { } secret)
        {
            payload["client_secret"] = secret;
        }

        using var response = await client
            .PostAsJsonAsync(tokenUrl, payload)
            .ConfigureAwait(false);

        if (!response.IsSuccessStatusCode)
        {
            var errorBody = await response.Content
                .ReadAsStringAsync()
                .ConfigureAwait(false);
            throw new InvalidOperationException(
                $"Auth0 token request failed ({response.StatusCode}): {errorBody}");
        }

        var tokenResponse = await response.Content
            .ReadFromJsonAsync(Auth0TokenJsonContext.Default.TokenResponse)
            .ConfigureAwait(false)
            ?? throw new InvalidOperationException("Auth0 returned an empty token response.");

        return tokenResponse.AccessToken;
    }

    internal sealed record TokenResponse(
        [property: JsonPropertyName("access_token")] string AccessToken);
}
