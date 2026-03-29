using Azure.Core;

namespace TownCrier.Infrastructure.Tests.Cosmos;

internal sealed class StubTokenCredential : TokenCredential
{
    public StubTokenCredential(string token, int expiresInMinutes = 60)
    {
        this.NextToken = token;
        this.ExpiresInMinutes = expiresInMinutes;
    }

    public string NextToken { get; set; }

    public int ExpiresInMinutes { get; set; }

    public override AccessToken GetToken(TokenRequestContext requestContext, CancellationToken cancellationToken) =>
        new(this.NextToken, DateTimeOffset.UtcNow.AddMinutes(this.ExpiresInMinutes));

    public override ValueTask<AccessToken> GetTokenAsync(TokenRequestContext requestContext, CancellationToken cancellationToken) =>
        ValueTask.FromResult(new AccessToken(this.NextToken, DateTimeOffset.UtcNow.AddMinutes(this.ExpiresInMinutes)));
}
