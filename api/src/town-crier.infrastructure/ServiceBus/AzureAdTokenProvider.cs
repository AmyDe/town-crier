using Azure.Core;

namespace TownCrier.Infrastructure.ServiceBus;

/// <summary>
/// Thin scoped token cache over an <see cref="TokenCredential"/>. Used by the
/// Service Bus data-plane and ARM management-plane clients, which need tokens
/// for different audiences ("https://servicebus.azure.net/.default" vs
/// "https://management.azure.com/.default"). One instance per audience.
/// </summary>
internal sealed class AzureAdTokenProvider : IDisposable
{
    private readonly string[] scopes;
    private readonly SemaphoreSlim refreshLock = new(1, 1);
    private readonly TokenCredential credential;
    private AccessToken cachedToken;

    public AzureAdTokenProvider(TokenCredential credential, string[] scopes)
    {
        ArgumentNullException.ThrowIfNull(credential);
        ArgumentNullException.ThrowIfNull(scopes);

        this.credential = credential;
        this.scopes = scopes;
    }

    public async Task<string> GetAuthorizationHeaderAsync(CancellationToken ct)
    {
        await this.refreshLock.WaitAsync(ct).ConfigureAwait(false);
        try
        {
            if (this.cachedToken.ExpiresOn <= DateTimeOffset.UtcNow.AddMinutes(5))
            {
                this.cachedToken = await this.credential.GetTokenAsync(
                    new TokenRequestContext(this.scopes), ct).ConfigureAwait(false);
            }

            return $"Bearer {this.cachedToken.Token}";
        }
        finally
        {
            this.refreshLock.Release();
        }
    }

    public void Dispose() => this.refreshLock.Dispose();
}
