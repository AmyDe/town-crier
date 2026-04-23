using System.Diagnostics;
using System.Net;
using System.Net.Http.Headers;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization.Metadata;
using TownCrier.Infrastructure.Observability;

namespace TownCrier.Infrastructure.Cosmos;

internal sealed class CosmosRestClient : ICosmosRestClient
{
    private const string ApiVersion = "2018-12-31";

    private readonly HttpClient httpClient;
    private readonly CosmosAuthProvider authProvider;
    private readonly string databaseName;

    public CosmosRestClient(
        HttpClient httpClient,
        CosmosAuthProvider authProvider,
        CosmosRestOptions options)
    {
        ArgumentNullException.ThrowIfNull(httpClient);
        ArgumentNullException.ThrowIfNull(authProvider);
        ArgumentNullException.ThrowIfNull(options);

        this.httpClient = httpClient;
        this.authProvider = authProvider;
        this.databaseName = options.DatabaseName;
    }

    public async Task<T?> ReadDocumentAsync<T>(
        string collection,
        string id,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos ReadItem");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "ReadItem");

#pragma warning disable CA2000 // using var disposes request on all paths
        using var request = this.BuildReadRequest(collection, id);
#pragma warning restore CA2000
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        RecordResponseMetrics(activity, response);

        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return default;
        }

        response.EnsureSuccessStatusCode();

        var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
        await using (stream.ConfigureAwait(false))
        {
            return await JsonSerializer.DeserializeAsync(stream, typeInfo, ct).ConfigureAwait(false);
        }
    }

    public async Task<CosmosReadResult<T>> ReadDocumentWithETagAsync<T>(
        string collection,
        string id,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos ReadItem");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "ReadItem");

#pragma warning disable CA2000 // using var disposes request on all paths
        using var request = this.BuildReadRequest(collection, id);
#pragma warning restore CA2000
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        RecordResponseMetrics(activity, response);

        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return new CosmosReadResult<T>(default, null);
        }

        response.EnsureSuccessStatusCode();

        var etag = response.Headers.ETag?.Tag;
        var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
        await using (stream.ConfigureAwait(false))
        {
            var document = await JsonSerializer.DeserializeAsync(stream, typeInfo, ct)
                .ConfigureAwait(false);
            return new CosmosReadResult<T>(document, etag);
        }
    }

    public async Task UpsertDocumentAsync<T>(
        string collection,
        T document,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos Upsert");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "Upsert");

        using var request = this.BuildCreateRequest(collection, document, typeInfo);
        request.Headers.TryAddWithoutValidation("x-ms-documentdb-is-upsert", "True");
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        RecordResponseMetrics(activity, response);

        response.EnsureSuccessStatusCode();
    }

    public async Task<bool> TryCreateDocumentAsync<T>(
        string collection,
        T document,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos TryCreate");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "TryCreate");

        using var request = this.BuildCreateRequest(collection, document, typeInfo);
        request.Headers.TryAddWithoutValidation("If-None-Match", "*");
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        RecordResponseMetrics(activity, response);

        if (response.StatusCode == HttpStatusCode.Created)
        {
            return true;
        }

        if (response.StatusCode == HttpStatusCode.Conflict)
        {
            return false;
        }

        await ThrowOnFailureAsync(response, "TryCreate", ct).ConfigureAwait(false);
        return false; // unreachable
    }

    public async Task<bool> TryReplaceDocumentAsync<T>(
        string collection,
        T document,
        string partitionKey,
        string ifMatchEtag,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrEmpty(ifMatchEtag);

        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos TryReplace");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "TryReplace");

        using var request = this.BuildReplaceRequest(collection, document, typeInfo);
        request.Headers.TryAddWithoutValidation("If-Match", ifMatchEtag);
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        RecordResponseMetrics(activity, response);

        if (response.IsSuccessStatusCode)
        {
            return true;
        }

        if (response.StatusCode == HttpStatusCode.PreconditionFailed)
        {
            return false;
        }

        await ThrowOnFailureAsync(response, "TryReplace", ct).ConfigureAwait(false);
        return false; // unreachable
    }

    public async Task DeleteDocumentAsync(
        string collection,
        string id,
        string partitionKey,
        CancellationToken ct)
    {
        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos Delete");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "Delete");

        using var request = this.BuildDeleteRequest(collection, id);
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        RecordResponseMetrics(activity, response);

        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return; // Idempotent delete
        }

        response.EnsureSuccessStatusCode();
    }

    public async Task<CosmosDeleteOutcome> TryDeleteDocumentAsync(
        string collection,
        string id,
        string partitionKey,
        string? ifMatchEtag,
        CancellationToken ct)
    {
        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos TryDelete");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "TryDelete");

        using var request = this.BuildDeleteRequest(collection, id);
        if (!string.IsNullOrEmpty(ifMatchEtag))
        {
            request.Headers.TryAddWithoutValidation("If-Match", ifMatchEtag);
        }

        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        RecordResponseMetrics(activity, response);

        if (response.StatusCode == HttpStatusCode.NoContent
            || response.StatusCode == HttpStatusCode.OK)
        {
            return CosmosDeleteOutcome.Deleted;
        }

        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return CosmosDeleteOutcome.NotFound;
        }

        if (response.StatusCode == HttpStatusCode.PreconditionFailed)
        {
            return CosmosDeleteOutcome.PreconditionFailed;
        }

        await ThrowOnFailureAsync(response, "TryDelete", ct).ConfigureAwait(false);
        return CosmosDeleteOutcome.Deleted; // unreachable
    }

    public async Task<List<T>> QueryAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos Query");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "Query");

        if (partitionKey is null && RequiresFanOut(sql))
        {
            return await this.QueryWithFanOutAsync(
                collection, sql, parameters, typeInfo, activity, ct).ConfigureAwait(false);
        }

        var results = new List<T>();
        string? continuation = null;

        do
        {
#pragma warning disable CA2000 // using var disposes request on all paths including early return
            using var request = this.BuildQueryRequest(collection, sql, parameters);
#pragma warning restore CA2000
            await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);
            AddQueryHeaders(request, partitionKey);

            if (continuation is not null)
            {
                request.Headers.TryAddWithoutValidation("x-ms-continuation", continuation);
            }

            using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

            // Fan-out detection: the gateway returns 400 on the initial probe when it
            // cannot serve DISTINCT/GROUP BY across partitions. Subsequent continuation
            // pages never trigger this — only the first request can.
            if (continuation is null
                && response.StatusCode == HttpStatusCode.BadRequest
                && partitionKey is null)
            {
                var errorBody = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
                if (errorBody.Contains("partitionedQueryExecutionInfoVersion", StringComparison.Ordinal))
                {
                    return await this.QueryWithFanOutAsync(
                        collection, sql, parameters, typeInfo, activity, ct).ConfigureAwait(false);
                }

                throw new HttpRequestException(
                    $"Cosmos DB query failed ({(int)response.StatusCode}): {errorBody} | SQL: {sql}");
            }

            await ThrowOnCosmosErrorAsync(response, sql, ct).ConfigureAwait(false);

            RecordResponseMetrics(activity, response);

            var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
            await using (stream.ConfigureAwait(false))
            {
                using var doc = await JsonDocument.ParseAsync(stream, cancellationToken: ct)
                    .ConfigureAwait(false);

                foreach (var element in doc.RootElement.GetProperty("Documents").EnumerateArray())
                {
                    results.Add(element.Deserialize(typeInfo)!);
                }
            }

            continuation = response.Headers.TryGetValues("x-ms-continuation", out var values)
                ? values.FirstOrDefault()
                : null;
        }
        while (continuation is not null);

        return results;
    }

    public async Task<T> ScalarQueryAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        var results = await this.QueryAsync(collection, sql, parameters, partitionKey, typeInfo, ct)
            .ConfigureAwait(false);
        return results.Count > 0
            ? results[0]
            : throw new InvalidOperationException("Query returned no results.");
    }

    public async Task<PagedQueryResult<T>> QueryPageAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        int maxItemCount,
        string? continuationToken,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        using var activity = CosmosInstrumentation.Source.StartActivity("Cosmos QueryPage");
        activity?.SetTag("db.system", "cosmosdb");
        activity?.SetTag("db.cosmosdb.container", collection);
        activity?.SetTag("db.operation.name", "QueryPage");

#pragma warning disable CA2000 // using var disposes request on all paths
        using var request = this.BuildQueryRequest(collection, sql, parameters);
#pragma warning restore CA2000
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);
        AddQueryHeaders(request, partitionKey);
        request.Headers.TryAddWithoutValidation(
            "x-ms-max-item-count", maxItemCount.ToString(System.Globalization.CultureInfo.InvariantCulture));

        if (continuationToken is not null)
        {
            request.Headers.TryAddWithoutValidation("x-ms-continuation", continuationToken);
        }

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);
        await ThrowOnCosmosErrorAsync(response, sql, ct).ConfigureAwait(false);
        RecordResponseMetrics(activity, response);

        var items = new List<T>();
        var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
        await using (stream.ConfigureAwait(false))
        {
            using var doc = await JsonDocument.ParseAsync(stream, cancellationToken: ct)
                .ConfigureAwait(false);

            foreach (var element in doc.RootElement.GetProperty("Documents").EnumerateArray())
            {
                items.Add(element.Deserialize(typeInfo)!);
            }
        }

        var nextContinuation = response.Headers.TryGetValues("x-ms-continuation", out var values)
            ? values.FirstOrDefault()
            : null;

        return new PagedQueryResult<T>(items, nextContinuation);
    }

    private static bool RequiresFanOut(string sql) =>
        sql.Contains("DISTINCT", StringComparison.OrdinalIgnoreCase) ||
        sql.Contains("GROUP BY", StringComparison.OrdinalIgnoreCase);

    private static void AddQueryHeaders(HttpRequestMessage request, string? partitionKey)
    {
        request.Headers.TryAddWithoutValidation("x-ms-documentdb-isquery", "True");

        if (partitionKey is null)
        {
            request.Headers.TryAddWithoutValidation(
                "x-ms-documentdb-query-enablecrosspartition",
                "True");
        }
    }

    private static async Task ThrowOnCosmosErrorAsync(
        HttpResponseMessage response,
        string sql,
        CancellationToken ct)
    {
        if (response.IsSuccessStatusCode)
        {
            return;
        }

        var body = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
        throw new HttpRequestException(
            $"Cosmos DB query failed ({(int)response.StatusCode}): {body} | SQL: {sql}");
    }

    private static void RecordResponseMetrics(Activity? activity, HttpResponseMessage response)
    {
        var statusCode = (int)response.StatusCode;
        activity?.SetTag("db.cosmosdb.status_code", statusCode);

        if (response.Headers.TryGetValues("x-ms-request-charge", out var ruValues))
        {
            var ruString = ruValues.FirstOrDefault();
            if (ruString is not null && double.TryParse(ruString, out var ru))
            {
                activity?.SetTag("db.cosmosdb.request_charge", ru);
                CosmosInstrumentation.RequestCharge.Record(ru);
            }
        }

        if (statusCode == 429)
        {
            CosmosInstrumentation.Throttles.Add(1);
        }
    }

    private static async Task ThrowOnFailureAsync(
        HttpResponseMessage response,
        string operation,
        CancellationToken ct)
    {
        if (!response.IsSuccessStatusCode)
        {
            var body = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
            throw new HttpRequestException(
                $"Cosmos DB {operation} failed ({(int)response.StatusCode}): {body}");
        }
    }

    private static string GetDocumentId<T>(T document, JsonTypeInfo<T> typeInfo)
    {
        // Serialize to JsonElement to extract the id field
        using var jsonDoc = JsonDocument.Parse(
            JsonSerializer.Serialize(document, typeInfo));
        return jsonDoc.RootElement.GetProperty("id").GetString()
            ?? throw new InvalidOperationException("Document must have an id field");
    }

    private async Task<List<T>> QueryWithFanOutAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        JsonTypeInfo<T> typeInfo,
        Activity? activity,
        CancellationToken ct)
    {
        var rangeIds = await this.GetPartitionKeyRangesAsync(collection, ct).ConfigureAwait(false);
        var results = new List<T>();

        foreach (var rangeId in rangeIds)
        {
            string? continuation = null;

            do
            {
                using var request = this.BuildQueryRequest(collection, sql, parameters);
                await this.AddHeadersAsync(request, null, ct).ConfigureAwait(false);
                AddQueryHeaders(request, null);
                request.Headers.TryAddWithoutValidation(
                    "x-ms-documentdb-partitionkeyrangeid", rangeId);

                if (continuation is not null)
                {
                    request.Headers.TryAddWithoutValidation("x-ms-continuation", continuation);
                }

                using var response = await this.httpClient.SendAsync(request, ct)
                    .ConfigureAwait(false);
                await ThrowOnCosmosErrorAsync(response, sql, ct).ConfigureAwait(false);

                RecordResponseMetrics(activity, response);

                var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
                await using (stream.ConfigureAwait(false))
                {
                    using var doc = await JsonDocument.ParseAsync(stream, cancellationToken: ct)
                        .ConfigureAwait(false);

                    foreach (var element in doc.RootElement.GetProperty("Documents").EnumerateArray())
                    {
                        results.Add(element.Deserialize(typeInfo)!);
                    }
                }

                continuation = response.Headers.TryGetValues("x-ms-continuation", out var values)
                    ? values.FirstOrDefault()
                    : null;
            }
            while (continuation is not null);
        }

        return results;
    }

    private async Task<List<string>> GetPartitionKeyRangesAsync(
        string collection,
        CancellationToken ct)
    {
        var resourceLink = $"dbs/{this.databaseName}/colls/{collection}/pkranges";
        using var request = new HttpRequestMessage(HttpMethod.Get, $"/{resourceLink}");
        await this.AddHeadersAsync(request, null, ct).ConfigureAwait(false);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        if (!response.IsSuccessStatusCode)
        {
            var body = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
            throw new HttpRequestException(
                $"Failed to fetch partition key ranges ({(int)response.StatusCode}): {body}");
        }

        var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
        await using (stream.ConfigureAwait(false))
        {
            using var doc = await JsonDocument.ParseAsync(stream, cancellationToken: ct)
                .ConfigureAwait(false);

            var ranges = new List<string>();
            foreach (var range in doc.RootElement.GetProperty("PartitionKeyRanges").EnumerateArray())
            {
                ranges.Add(range.GetProperty("id").GetString()!);
            }

            return ranges;
        }
    }

    private HttpRequestMessage BuildReadRequest(string collection, string id)
    {
        var encodedId = Uri.EscapeDataString(id);
        var resourceLink = $"dbs/{this.databaseName}/colls/{collection}/docs/{encodedId}";
        return new HttpRequestMessage(HttpMethod.Get, $"/{resourceLink}");
    }

    private HttpRequestMessage BuildDeleteRequest(string collection, string id)
    {
        var encodedId = Uri.EscapeDataString(id);
        var resourceLink = $"dbs/{this.databaseName}/colls/{collection}/docs/{encodedId}";
        return new HttpRequestMessage(HttpMethod.Delete, $"/{resourceLink}");
    }

    private HttpRequestMessage BuildQueryRequest(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters)
    {
        var resourceLink = $"dbs/{this.databaseName}/colls/{collection}";
        var request = new HttpRequestMessage(HttpMethod.Post, $"/{resourceLink}/docs");

        var queryParameters = parameters?.Select(p =>
            new CosmosQueryParameter(p.Name, p.Value)).ToList()
            ?? [];
        var body = new CosmosQueryBody(sql, queryParameters);

        var json = JsonSerializer.Serialize(body, CosmosJsonSerializerContext.Default.CosmosQueryBody);
        request.Content = new StringContent(json, Encoding.UTF8);
        request.Content.Headers.ContentType = new MediaTypeHeaderValue("application/query+json");

        return request;
    }

    private HttpRequestMessage BuildCreateRequest<T>(
        string collection,
        T document,
        JsonTypeInfo<T> typeInfo)
    {
        var resourceLink = $"dbs/{this.databaseName}/colls/{collection}";
        var request = new HttpRequestMessage(HttpMethod.Post, $"/{resourceLink}/docs");

        request.Content = new StringContent(
            JsonSerializer.Serialize(document, typeInfo),
            Encoding.UTF8,
            "application/json");

        return request;
    }

    private HttpRequestMessage BuildReplaceRequest<T>(
        string collection,
        T document,
        JsonTypeInfo<T> typeInfo)
    {
        var documentId = GetDocumentId(document, typeInfo);
        var encodedId = Uri.EscapeDataString(documentId);
        var resourceLink = $"dbs/{this.databaseName}/colls/{collection}/docs/{encodedId}";
        var request = new HttpRequestMessage(HttpMethod.Put, $"/{resourceLink}");

        request.Content = new StringContent(
            JsonSerializer.Serialize(document, typeInfo),
            Encoding.UTF8,
            "application/json");

        return request;
    }

    private async Task AddHeadersAsync(
        HttpRequestMessage request,
        string? partitionKey,
        CancellationToken ct)
    {
        var date = DateTime.UtcNow.ToString("R");
        var auth = await this.authProvider.GetAuthorizationHeaderAsync(ct).ConfigureAwait(false);

        request.Headers.TryAddWithoutValidation("Authorization", auth);
        request.Headers.TryAddWithoutValidation("x-ms-date", date);
        request.Headers.TryAddWithoutValidation("x-ms-version", ApiVersion);

        if (partitionKey is not null)
        {
            var escapedKey = JsonSerializer.Serialize(
                partitionKey, CosmosJsonSerializerContext.Default.String);
            request.Headers.TryAddWithoutValidation(
                "x-ms-documentdb-partitionkey",
                $"[{escapedKey}]");
        }
    }
}
