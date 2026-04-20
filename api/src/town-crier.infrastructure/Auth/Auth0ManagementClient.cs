using System.Net.Http.Json;
using TownCrier.Application.Auth;

namespace TownCrier.Infrastructure.Auth;

public sealed class Auth0ManagementClient : IAuth0ManagementClient
{
    private readonly HttpClient httpClient;
    private readonly string domain;
    private readonly string clientId;
    private readonly string clientSecret;
    private string? cachedToken;
    private DateTimeOffset tokenExpiry;

    public Auth0ManagementClient(HttpClient httpClient, string domain, string clientId, string clientSecret)
    {
        this.httpClient = httpClient;
        this.domain = domain;
        this.clientId = clientId;
        this.clientSecret = clientSecret;
    }

    public async Task UpdateSubscriptionTierAsync(string userId, string tier, CancellationToken ct)
    {
        var token = await this.GetTokenAsync(ct).ConfigureAwait(false);

        var url = $"https://{this.domain}/api/v2/users/{Uri.EscapeDataString(userId)}";
        using var request = new HttpRequestMessage(HttpMethod.Patch, url);
        request.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);
        request.Content = JsonContent.Create(
            new Auth0UpdateMetadataRequest(new Auth0AppMetadata(tier)),
            Auth0ManagementClientJsonSerializerContext.Default.Auth0UpdateMetadataRequest);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);
        response.EnsureSuccessStatusCode();
    }

    public async Task DeleteUserAsync(string userId, CancellationToken ct)
    {
        var token = await this.GetTokenAsync(ct).ConfigureAwait(false);

        var url = $"https://{this.domain}/api/v2/users/{Uri.EscapeDataString(userId)}";
        using var request = new HttpRequestMessage(HttpMethod.Delete, url);
        request.Headers.Authorization = new System.Net.Http.Headers.AuthenticationHeaderValue("Bearer", token);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);
        if (response.StatusCode == System.Net.HttpStatusCode.NotFound)
        {
            return;
        }

        response.EnsureSuccessStatusCode();
    }

    private async Task<string> GetTokenAsync(CancellationToken ct)
    {
        if (this.cachedToken is not null && DateTimeOffset.UtcNow < this.tokenExpiry)
        {
            return this.cachedToken;
        }

        using var request = new HttpRequestMessage(HttpMethod.Post, $"https://{this.domain}/oauth/token");
        request.Content = JsonContent.Create(
            new Auth0TokenRequest(
                GrantType: "client_credentials",
                ClientId: this.clientId,
                ClientSecret: this.clientSecret,
                Audience: $"https://{this.domain}/api/v2/"),
            Auth0ManagementClientJsonSerializerContext.Default.Auth0TokenRequest);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);
        response.EnsureSuccessStatusCode();

        var tokenResponse = await response.Content.ReadFromJsonAsync(
            Auth0ManagementClientJsonSerializerContext.Default.Auth0TokenResponse, ct).ConfigureAwait(false);

        this.cachedToken = tokenResponse!.AccessToken;
        this.tokenExpiry = DateTimeOffset.UtcNow.AddSeconds(tokenResponse.ExpiresIn - 60);
        return this.cachedToken;
    }
}
