using System.Net;
using Azure.Core;

namespace TownCrier.Infrastructure.Cosmos;

internal sealed class CosmosAuthProvider
{
    private static readonly string[] Scopes = ["https://cosmos.azure.com/.default"];
    private readonly TokenCredential credential;
    private AccessToken cachedToken;

    public CosmosAuthProvider(TokenCredential credential)
    {
        ArgumentNullException.ThrowIfNull(credential);
        this.credential = credential;
    }

    public async Task<string> GetAuthorizationHeaderAsync(CancellationToken ct)
    {
        if (this.cachedToken.ExpiresOn <= DateTimeOffset.UtcNow.AddMinutes(5))
        {
            this.cachedToken = await this.credential.GetTokenAsync(
                new TokenRequestContext(Scopes), ct).ConfigureAwait(false);
        }

        // Cosmos DB Entra ID auth format: type=aad&ver=1.0&sig={token} (URL-encoded)
        return WebUtility.UrlEncode($"type=aad&ver=1.0&sig={this.cachedToken.Token}");
    }
}
