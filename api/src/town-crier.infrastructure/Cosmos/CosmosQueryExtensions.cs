using Microsoft.Azure.Cosmos;

namespace TownCrier.Infrastructure.Cosmos;

/// <summary>
/// Extension methods for <see cref="FeedIterator{T}"/> that eliminate
/// repetitive query-iteration boilerplate across Cosmos DB repositories.
/// </summary>
internal static class CosmosQueryExtensions
{
    /// <summary>
    /// Drains all pages from the iterator, applying <paramref name="map"/> to each item,
    /// and returns the collected results as a list.
    /// </summary>
    public static async Task<List<TResult>> CollectAsync<TDocument, TResult>(
        this FeedIterator<TDocument> iterator,
        Func<TDocument, TResult> map,
        CancellationToken ct)
    {
        var results = new List<TResult>();

        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            results.AddRange(response.Select(map));
        }

        return results;
    }

    /// <summary>
    /// Drains all pages from the iterator and returns the raw items as a list (no mapping).
    /// Useful when the query already returns the desired type (e.g. scalar projections).
    /// </summary>
    public static async Task<List<T>> CollectAsync<T>(
        this FeedIterator<T> iterator,
        CancellationToken ct)
    {
        var results = new List<T>();

        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            results.AddRange(response);
        }

        return results;
    }

    /// <summary>
    /// Returns the first item from the iterator after applying <paramref name="map"/>,
    /// or <c>default</c> if no items are returned. Stops iterating after the first match.
    /// </summary>
    public static async Task<TResult?> FirstOrDefaultAsync<TDocument, TResult>(
        this FeedIterator<TDocument> iterator,
        Func<TDocument, TResult> map,
        CancellationToken ct)
    {
        while (iterator.HasMoreResults)
        {
            var response = await iterator.ReadNextAsync(ct).ConfigureAwait(false);
            var document = response.FirstOrDefault();
            if (document is not null)
            {
                return map(document);
            }
        }

        return default;
    }
}
