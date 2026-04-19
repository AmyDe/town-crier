using TownCrier.Infrastructure.Authorities;

namespace TownCrier.Infrastructure.Tests.Authorities;

public sealed class AllAuthorityIdProviderTests
{
    [Test]
    public async Task Should_ReturnAllAuthorityIds_When_Queried()
    {
        // Arrange
        var authorityProvider = new StaticAuthorityProvider();
        var all = await authorityProvider.GetAllAsync(CancellationToken.None);
        var provider = new AllAuthorityIdProvider(authorityProvider);

        // Act
        var result = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert — count matches the embedded authority list
        await Assert.That(result).HasCount().EqualTo(all.Count);
    }

    [Test]
    public async Task Should_ReturnDistinctIds_When_Queried()
    {
        // Arrange
        var authorityProvider = new StaticAuthorityProvider();
        var provider = new AllAuthorityIdProvider(authorityProvider);

        // Act
        var result = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result.Distinct().Count()).IsEqualTo(result.Count);
    }
}
