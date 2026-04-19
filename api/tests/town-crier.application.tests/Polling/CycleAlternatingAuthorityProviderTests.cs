using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class CycleAlternatingAuthorityProviderTests
{
    [Test]
    public async Task Should_DelegateToWatchedProvider_When_CycleIsWatched()
    {
        // Arrange
        var watched = new FakeWatchZoneActiveAuthorityProvider();
        watched.Add(100);
        watched.Add(200);
        var all = new FakeAllAuthorityIdProvider();
        all.Add(300);
        all.Add(400);
        var selector = new FakeCycleSelector(CycleType.Watched);
        var provider = new CycleAlternatingAuthorityProvider(watched, all, selector);

        // Act
        var result = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(2);
        await Assert.That(result).Contains(100);
        await Assert.That(result).Contains(200);
    }

    [Test]
    public async Task Should_DelegateToAllProvider_When_CycleIsSeed()
    {
        // Arrange
        var watched = new FakeWatchZoneActiveAuthorityProvider();
        watched.Add(100);
        var all = new FakeAllAuthorityIdProvider();
        all.Add(300);
        all.Add(400);
        all.Add(500);
        var selector = new FakeCycleSelector(CycleType.Seed);
        var provider = new CycleAlternatingAuthorityProvider(watched, all, selector);

        // Act
        var result = await provider.GetActiveAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(3);
        await Assert.That(result).Contains(300);
        await Assert.That(result).Contains(400);
        await Assert.That(result).Contains(500);
    }
}
