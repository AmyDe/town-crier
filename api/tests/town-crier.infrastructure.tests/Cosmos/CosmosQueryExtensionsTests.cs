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

    [Test]
    public async Task Should_ReturnAllMappedItems_When_MultiplePages()
    {
        // Arrange — three pages of results
        var pages = new[]
        {
            new[] { "alpha", "bravo" },
            new[] { "charlie" },
            new[] { "delta", "echo" },
        };
        using var iterator = new FakeFeedIterator<string>(pages);

        // Act
        var results = await iterator.CollectAsync(s => s.ToUpperInvariant(), CancellationToken.None);

        // Assert
        await Assert.That(results).HasCount().EqualTo(5);
        await Assert.That(results[0]).IsEqualTo("ALPHA");
        await Assert.That(results[1]).IsEqualTo("BRAVO");
        await Assert.That(results[2]).IsEqualTo("CHARLIE");
        await Assert.That(results[3]).IsEqualTo("DELTA");
        await Assert.That(results[4]).IsEqualTo("ECHO");
    }

    [Test]
    public async Task Should_ReturnEmptyList_When_NoResults()
    {
        // Arrange — no pages at all
        using var iterator = new FakeFeedIterator<string>(Array.Empty<IReadOnlyList<string>>());

        // Act
        var results = await iterator.CollectAsync(s => s.ToUpperInvariant(), CancellationToken.None);

        // Assert
        await Assert.That(results).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ReturnAllItems_When_NoMapperProvided()
    {
        // Arrange — identity collect (no mapping function)
        var pages = new[] { new[] { 10, 20 }, new[] { 30 } };
        using var iterator = new FakeFeedIterator<int>(pages);

        // Act
        var results = await iterator.CollectAsync(CancellationToken.None);

        // Assert
        await Assert.That(results).HasCount().EqualTo(3);
        await Assert.That(results[0]).IsEqualTo(10);
        await Assert.That(results[1]).IsEqualTo(20);
        await Assert.That(results[2]).IsEqualTo(30);
    }

    [Test]
    public async Task Should_ReturnFirstMappedItem_When_ResultExists()
    {
        // Arrange
        var pages = new[] { new[] { "alpha", "bravo" } };
        using var iterator = new FakeFeedIterator<string>(pages);

        // Act
        var result = await iterator.FirstOrDefaultAsync(s => s.ToUpperInvariant(), CancellationToken.None);

        // Assert
        await Assert.That(result).IsEqualTo("ALPHA");
    }

    [Test]
    public async Task Should_ReturnNull_When_NoResultsForFirstOrDefault()
    {
        // Arrange — empty iterator
        using var iterator = new FakeFeedIterator<string>(Array.Empty<IReadOnlyList<string>>());

        // Act
        var result = await iterator.FirstOrDefaultAsync(s => s.ToUpperInvariant(), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_ReturnFirstMappedItem_When_SpreadAcrossMultiplePages()
    {
        // Arrange — first page is empty, second page has data
        // This simulates an edge case where Cosmos returns an empty page
        // before a page with actual results
        var pages = new IReadOnlyList<string>[]
        {
            Array.Empty<string>(),
            new[] { "found-it" },
        };
        using var iterator = new FakeFeedIterator<string>(pages);

        // Act
        var result = await iterator.FirstOrDefaultAsync(s => s.ToUpperInvariant(), CancellationToken.None);

        // Assert
        await Assert.That(result).IsEqualTo("FOUND-IT");
    }

    [Test]
    public async Task Should_ReturnScalarValue_When_QueryReturnsValue()
    {
        // Arrange — simulates SELECT VALUE COUNT(1) FROM c which returns a single int
        var pages = new[] { new[] { 42 } };
        using var iterator = new FakeFeedIterator<int>(pages);

        // Act
        var result = await iterator.ScalarAsync(CancellationToken.None);

        // Assert
        await Assert.That(result).IsEqualTo(42);
    }

    [Test]
    public async Task Should_ReturnDefault_When_ScalarQueryReturnsNoResults()
    {
        // Arrange
        using var iterator = new FakeFeedIterator<int>(Array.Empty<IReadOnlyList<int>>());

        // Act
        var result = await iterator.ScalarAsync(CancellationToken.None);

        // Assert
        await Assert.That(result).IsEqualTo(0);
    }
}
