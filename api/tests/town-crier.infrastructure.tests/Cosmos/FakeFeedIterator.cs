using Microsoft.Azure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

/// <summary>
/// A hand-written fake for <see cref="FeedIterator{T}"/> that returns
/// pre-configured pages of results, simulating Cosmos DB paging behavior.
/// </summary>
/// <typeparam name="T">The item type returned per page.</typeparam>
internal sealed class FakeFeedIterator<T> : FeedIterator<T>
{
    private readonly Queue<FakeFeedResponse<T>> pages;

    public FakeFeedIterator(IEnumerable<IReadOnlyList<T>> pages)
    {
        this.pages = new Queue<FakeFeedResponse<T>>(
            pages.Select(p => new FakeFeedResponse<T>(p)));
    }

    public override bool HasMoreResults => this.pages.Count > 0;

    public override Task<FeedResponse<T>> ReadNextAsync(CancellationToken cancellationToken = default)
    {
        if (this.pages.Count == 0)
        {
            throw new InvalidOperationException("No more pages available.");
        }

        return Task.FromResult<FeedResponse<T>>(this.pages.Dequeue());
    }
}
