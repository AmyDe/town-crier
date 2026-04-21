using TownCrier.Infrastructure.ServiceBus;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.ServiceBus;

public sealed class ServiceBusAuthProviderTests
{
    [Test]
    public async Task Should_ReturnBearerFormat_When_GettingAuthorizationHeader()
    {
        var credential = new StubTokenCredential("test-token-123");
        using var provider = new ServiceBusAuthProvider(credential);

        var header = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);

        await Assert.That(header).IsEqualTo("Bearer test-token-123");
    }

    [Test]
    public async Task Should_CacheToken_When_NotExpired()
    {
        var credential = new StubTokenCredential("token-1");
        using var provider = new ServiceBusAuthProvider(credential);

        var first = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);
        credential.NextToken = "token-2";
        var second = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);

        await Assert.That(second).IsEqualTo(first);
    }

    [Test]
    public async Task Should_RefreshToken_When_Expired()
    {
        var credential = new StubTokenCredential("token-1", expiresInMinutes: 0);
        using var provider = new ServiceBusAuthProvider(credential);

        var first = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);
        credential.NextToken = "token-2";
        credential.ExpiresInMinutes = 60;
        var second = await provider.GetAuthorizationHeaderAsync(CancellationToken.None);

        await Assert.That(first).Contains("token-1");
        await Assert.That(second).Contains("token-2");
    }
}
