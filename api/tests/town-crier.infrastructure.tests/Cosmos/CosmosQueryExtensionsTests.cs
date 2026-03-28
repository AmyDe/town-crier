using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class CosmosQueryExtensionsTests
{
    [Test]
    public async Task Should_ReturnAllMappedItems_When_SinglePage()
    {
        // Arrange
        var pages = new[] { new[] { "alpha", "bravo", "charlie" } };
        using var iterator = new FakeFeedIterator<string>(pages);

        // Act
        var results = await iterator.CollectAsync(s => s.ToUpperInvariant(), CancellationToken.None);

        // Assert
        await Assert.That(results).HasCount().EqualTo(3);
        await Assert.That(results[0]).IsEqualTo("ALPHA");
        await Assert.That(results[1]).IsEqualTo("BRAVO");
        await Assert.That(results[2]).IsEqualTo("CHARLIE");
    }
}
