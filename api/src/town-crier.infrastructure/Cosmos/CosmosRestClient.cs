using System.Net;
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
        var resourceLink = $"dbs/{this.databaseName}/colls/{collection}/docs/{id}";
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

    public Task UpsertDocumentAsync<T>(
        string collection,
        T document,
        string partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        throw new NotImplementedException();
    }

    public Task DeleteDocumentAsync(
        string collection,
        string id,
        string partitionKey,
        CancellationToken ct)
    {
        throw new NotImplementedException();
    }

    public Task<List<T>> QueryAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        throw new NotImplementedException();
    }

    public Task<T> ScalarQueryAsync<T>(
        string collection,
        string sql,
        IReadOnlyList<QueryParameter>? parameters,
        string? partitionKey,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        throw new NotImplementedException();
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
            request.Headers.TryAddWithoutValidation(
                "x-ms-documentdb-partitionkey",
                $"[\"{partitionKey}\"]");
        }
    }
}
