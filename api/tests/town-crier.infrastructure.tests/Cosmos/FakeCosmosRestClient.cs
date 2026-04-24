using System.Text.Json;
using System.Text.Json.Serialization.Metadata;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

/// <summary>
/// In-memory fake of <see cref="ICosmosRestClient"/> for testing Cosmos repository adapters.
/// Stores documents as raw JSON strings keyed by (collection, id, partitionKey).
/// </summary>
internal sealed class FakeCosmosRestClient : ICosmosRestClient
{
    private readonly Dictionary<(string Collection, string Id, string PartitionKey), string> store = new();
    private readonly Dictionary<(string Collection, string Id, string PartitionKey), string> etags = new();
    private readonly Dictionary<string, object> cannedQueryResults = new();
    private readonly Dictionary<string, (object Results, string? ContinuationToken)> cannedPageResults = new();
    private long nextEtag;

    public string? LastPageQuerySql { get; private set; }

    /// <summary>
    /// Registers a pre-canned result list that <see cref="QueryAsync{T}"/> will return
    /// when the SQL query starts with <paramref name="sqlPrefix"/>. This bypasses
    /// normal document-store deserialization and returns the list directly, allowing
    /// tests to simulate partition fan-out scenarios (duplicates, partial aggregates).
    /// </summary>
    /// <typeparam name="T">The element type of the result list.</typeparam>
    /// <param name="sqlPrefix">The SQL prefix to match against the query.</param>
    /// <param name="results">The pre-canned result list to return.</param>
    public void SetQueryResults<T>(string sqlPrefix, List<T> results)
    {
        this.cannedQueryResults[sqlPrefix] = results;
    }

    public void SetPageQueryResults<T>(string sqlPrefix, List<T> results, string? continuationToken = null)
    {
        this.cannedPageResults[sqlPrefix] = (results, continuationToken);
    }

    public Task<T?> ReadDocumentAsync<T>(
        string collection,
        string id,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        if (this.store.TryGetValue((collection, id, partitionKey), out var json))
        {
            var result = JsonSerializer.Deserialize(json, typeInfo);
            return Task.FromResult(result);
        }

        return Task.FromResult(default(T));
    }

    public Task<CosmosReadResult<T>> ReadDocumentWithETagAsync<T>(
        string collection,
        string id,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        var key = (collection, id, partitionKey);
        if (this.store.TryGetValue(key, out var json))
        {
            var result = JsonSerializer.Deserialize(json, typeInfo);
            this.etags.TryGetValue(key, out var etag);
            return Task.FromResult(new CosmosReadResult<T>(result, etag));
        }

        return Task.FromResult(new CosmosReadResult<T>(default, null));
    }

    public Task UpsertDocumentAsync<T>(
        string collection,
        T document,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        var json = JsonSerializer.Serialize(document, typeInfo);

        // Extract id from the serialized JSON to use as the key
        using var doc = JsonDocument.Parse(json);
        var id = ExtractId(doc);

        var key = (collection, id, partitionKey);
        this.store[key] = json;
        this.etags[key] = this.NewEtag();
        return Task.CompletedTask;
    }

    public Task<bool> TryCreateDocumentAsync<T>(
        string collection,
        T document,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        var json = JsonSerializer.Serialize(document, typeInfo);

        // Extract id from the serialized JSON to use as the key
        using var doc = JsonDocument.Parse(json);
        var id = ExtractId(doc);

        var key = (collection, id, partitionKey);

        // If-None-Match: * semantics — return false if document already exists
        if (this.store.ContainsKey(key))
        {
            return Task.FromResult(false);
        }

        // Store the document, assign a new ETag, and return true (created)
        this.store[key] = json;
        this.etags[key] = this.NewEtag();
        return Task.FromResult(true);
    }

    public Task<bool> TryReplaceDocumentAsync<T>(
        string collection,
        T document,
        string partitionKey,
        string ifMatchEtag,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        var json = JsonSerializer.Serialize(document, typeInfo);

        // Extract id from the serialized JSON to use as the key
        using var doc = JsonDocument.Parse(json);
        var id = ExtractId(doc);

        var key = (collection, id, partitionKey);

        // CAS: require current ETag to match ifMatchEtag. Missing document (no ETag
        // tracked) or mismatch returns false without mutating state.
        if (!this.etags.TryGetValue(key, out var current) || current != ifMatchEtag)
        {
            return Task.FromResult(false);
        }

        // ETag matched — overwrite and bump to a new ETag
        this.store[key] = json;
        this.etags[key] = this.NewEtag();
        return Task.FromResult(true);
    }

    public Task DeleteDocumentAsync(
        string collection,
        string id,
        string partitionKey,
        CancellationToken ct)
    {
        // Idempotent — no error if not found (matches REST API behavior)
        var key = (collection, id, partitionKey);
        this.store.Remove(key);
        this.etags.Remove(key);
        return Task.CompletedTask;
    }

    public Task<CosmosDeleteOutcome> TryDeleteDocumentAsync(
        string collection,
        string id,
        string partitionKey,
        string? ifMatchEtag,
        CancellationToken ct)
    {
        var key = (collection, id, partitionKey);

        if (!this.store.ContainsKey(key))
        {
            return Task.FromResult(CosmosDeleteOutcome.NotFound);
        }

        // CAS: when caller supplied an If-Match, require it to equal the tracked ETag.
        // Mismatch surfaces as PreconditionFailed without mutating state.
        if (ifMatchEtag is not null
            && this.etags.TryGetValue(key, out var current)
            && current != ifMatchEtag)
        {
            return Task.FromResult(CosmosDeleteOutcome.PreconditionFailed);
        }

        this.store.Remove(key);
        this.etags.Remove(key);
        return Task.FromResult(CosmosDeleteOutcome.Deleted);
    }

    public Task<List<T>> QueryAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        // If a pre-canned result was registered for this SQL prefix, return it directly.
        // This lets tests simulate partition fan-out scenarios (duplicates, partial aggregates).
        foreach (var (prefix, value) in this.cannedQueryResults)
        {
            if (sql.StartsWith(prefix, StringComparison.Ordinal) && value is List<T> canned)
            {
                return Task.FromResult(new List<T>(canned));
            }
        }

        // Return all documents in the collection, optionally filtered by partition key.
        // The fake does not parse SQL — tests must set up data so that
        // "return everything matching the partition" is a valid approximation.
        var results = new List<T>();

        foreach (var ((c, _, pk), json) in this.store)
        {
            if (c != collection)
            {
                continue;
            }

            if (partitionKey is not null && pk != partitionKey)
            {
                continue;
            }

            var item = JsonSerializer.Deserialize(json, typeInfo);
            if (item is not null)
            {
                results.Add(item);
            }
        }

        return Task.FromResult(results);
    }

    public Task<T> ScalarQueryAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        // For scalar queries (COUNT etc.), return the count of matching documents
        // wrapped as the requested type. This works for int counts.
        var count = 0;

        foreach (var ((c, _, pk), _) in this.store)
        {
            if (c != collection)
            {
                continue;
            }

            if (partitionKey is not null && pk != partitionKey)
            {
                continue;
            }

            count++;
        }

        // Convert count to T (works for int, long, etc.)
        var result = (T)(object)count;
        return Task.FromResult(result);
    }

    public Task<PagedQueryResult<T>> QueryPageAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        int maxItemCount,
        string? continuationToken,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        this.LastPageQuerySql = sql;

        foreach (var (prefix, (value, token)) in this.cannedPageResults)
        {
            if (sql.StartsWith(prefix, StringComparison.Ordinal) && value is List<T> canned)
            {
                return Task.FromResult(new PagedQueryResult<T>(canned, token));
            }
        }

        var results = new List<T>();
        foreach (var ((c, _, pk), json) in this.store)
        {
            if (c != collection)
            {
                continue;
            }

            if (partitionKey is not null && pk != partitionKey)
            {
                continue;
            }

            var item = JsonSerializer.Deserialize(json, typeInfo);
            if (item is not null)
            {
                results.Add(item);
            }
        }

        return Task.FromResult(new PagedQueryResult<T>(results, null));
    }

    private static string ExtractId(JsonDocument doc)
    {
        if (doc.RootElement.TryGetProperty("id", out var idProp))
        {
            return idProp.GetString()!;
        }

        if (doc.RootElement.TryGetProperty("Id", out var idProp2))
        {
            return idProp2.GetString()!;
        }

        throw new InvalidOperationException("Document must have an 'id' or 'Id' property.");
    }

    private string NewEtag() => $"\"v{Interlocked.Increment(ref this.nextEtag)}\"";
}
