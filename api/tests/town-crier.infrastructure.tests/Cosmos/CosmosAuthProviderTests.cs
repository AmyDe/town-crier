using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class CosmosAuthProviderTests
{
    [Test]
    public async Task Should_ReturnEntraIdFormat_When_GettingAuthorizationHeader()
    {
        var credential = new StubTokenCredential("test-token-123");
        var provider = new CosmosAuthProvider(credential);

        var header = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);

        await Assert.That(header).IsEqualTo("type%3Daad%26ver%3D1.0%26sig%3Dtest-token-123");
    }

    [Test]
    public async Task Should_CacheToken_When_NotExpired()
    {
        var credential = new StubTokenCredential("token-1");
        var provider = new CosmosAuthProvider(credential);

        var first = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);
        credential.NextToken = "token-2";
        var second = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);

        // Same token because it's cached (not expired)
        await Assert.That(second).IsEqualTo(first);
    }

    [Test]
    public async Task Should_RefreshToken_When_Expired()
    {
        var credential = new StubTokenCredential("token-1", expiresInMinutes: 0);
        var provider = new CosmosAuthProvider(credential);

        var first = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);
        credential.NextToken = "token-2";
        credential.ExpiresInMinutes = 60;
        var second = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);

        await Assert.That(first).Contains("token-1");
        await Assert.That(second).Contains("token-2");
    }
}
