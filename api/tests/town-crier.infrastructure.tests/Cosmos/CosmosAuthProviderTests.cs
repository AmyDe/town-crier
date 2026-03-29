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
}
