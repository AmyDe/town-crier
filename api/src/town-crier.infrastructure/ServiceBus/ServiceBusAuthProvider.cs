using Azure.Core;

namespace TownCrier.Infrastructure.ServiceBus;

internal sealed class ServiceBusAuthProvider : IDisposable
{
    private static readonly string[] Scopes = ["https://servicebus.azure.net/.default"];
    private readonly SemaphoreSlim refreshLock = new(1, 1);
    private readonly TokenCredential credential;
    private AccessToken cachedToken;

    public ServiceBusAuthProvider(TokenCredential credential)
    {
        ArgumentNullException.ThrowIfNull(credential);
        this.credential = credential;
    }

    public async Task<string> GetAuthorizationHeaderAsync(CancellationToken ct)
    {
        await this.refreshLock.WaitAsync(ct).ConfigureAwait(false);
        try
        {
            if (this.cachedToken.ExpiresOn <= DateTimeOffset.UtcNow.AddMinutes(5))
            {
                this.cachedToken = await this.credential.GetTokenAsync(
                    new TokenRequestContext(Scopes), ct).ConfigureAwait(false);
            }

            // Service Bus REST auth format: plain bearer token (no SAS/aad wrapper).
            return $"Bearer {this.cachedToken.Token}";
        }
        finally
        {
            this.refreshLock.Release();
        }
    }

    public void Dispose() => this.refreshLock.Dispose();
}
