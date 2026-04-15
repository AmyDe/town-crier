using System.Text.Json.Serialization.Metadata;

namespace TownCrier.Infrastructure.Cosmos;

public interface ICosmosRestClient
{
    Task<T?> ReadDocumentAsync<T>(
        string collection,
        string id,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct);

    Task UpsertDocumentAsync<T>(
        string collection,
        T document,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct);

    Task DeleteDocumentAsync(
        string collection,
        string id,
        string partitionKey,
        CancellationToken ct);

    Task<List<T>> QueryAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct);

    Task<T> ScalarQueryAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct);

    Task<PagedQueryResult<T>> QueryPageAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        int maxItemCount,
        string? continuationToken,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct);
}

public readonly record struct QueryParameter(string Name, object Value);
