using System.Net;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization.Metadata;

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
        var encodedId = Uri.EscapeDataString(id);
        var resourceLink = $"dbs/{this.databaseName}/colls/{collection}/docs/{encodedId}";
        using var request = new HttpRequestMessage(HttpMethod.Get, $"/{resourceLink}");
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

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

    public async Task UpsertDocumentAsync<T>(
        string collection,
        T document,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        var resourceLink = $"dbs/{this.databaseName}/colls/{collection}";
        using var request = new HttpRequestMessage(HttpMethod.Post, $"/{resourceLink}/docs");
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);
        request.Headers.TryAddWithoutValidation("x-ms-documentdb-is-upsert", "True");

        request.Content = new StringContent(
            JsonSerializer.Serialize(document, typeInfo),
            Encoding.UTF8,
            "application/json");

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);
        response.EnsureSuccessStatusCode();
    }

    public async Task DeleteDocumentAsync(
        string collection,
        string id,
        string partitionKey,
        CancellationToken ct)
    {
        var encodedId = Uri.EscapeDataString(id);
        var resourceLink = $"dbs/{this.databaseName}/colls/{collection}/docs/{encodedId}";
        using var request = new HttpRequestMessage(HttpMethod.Delete, $"/{resourceLink}");
        await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);

        using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);

        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return; // Idempotent delete
        }

        response.EnsureSuccessStatusCode();
    }

    public async Task<List<T>> QueryAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        var results = new List<T>();
        string? continuation = null;

        do
        {
            using var request = this.BuildQueryRequest(collection, sql, parameters);
            await this.AddHeadersAsync(request, partitionKey, ct).ConfigureAwait(false);
            AddQueryHeaders(request, partitionKey);

            if (continuation is not null)
            {
                request.Headers.TryAddWithoutValidation("x-ms-continuation", continuation);
            }

            using var response = await this.httpClient.SendAsync(request, ct).ConfigureAwait(false);
            await ThrowOnCosmosErrorAsync(response, sql, ct).ConfigureAwait(false);

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

        request.Content = new StringContent(
            JsonSerializer.Serialize(body, CosmosJsonSerializerContext.Default.CosmosQueryBody),
            Encoding.UTF8,
            "application/query+json");

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
