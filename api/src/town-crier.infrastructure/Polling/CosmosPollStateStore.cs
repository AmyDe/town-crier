using System.Globalization;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Polling;

public sealed class CosmosPollStateStore : IPollStateStore
{
    private const string GlobalDocumentId = "poll-state";

    private readonly ICosmosRestClient client;

    public CosmosPollStateStore(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<DateTimeOffset?> GetLastPollTimeAsync(int authorityId, CancellationToken ct)
    {
        var documentId = FormatDocumentId(authorityId);
        var doc = await this.client.ReadDocumentAsync(
            CosmosContainerNames.PollState,
            documentId,
            documentId,
            CosmosJsonSerializerContext.Default.PollStateDocument,
            ct).ConfigureAwait(false);

        if (doc is null)
        {
            return null;
        }

        return DateTimeOffset.Parse(doc.LastPollTime, CultureInfo.InvariantCulture);
    }

    public async Task SaveLastPollTimeAsync(int authorityId, DateTimeOffset pollTime, CancellationToken ct)
    {
        var documentId = FormatDocumentId(authorityId);
        var doc = new PollStateDocument
        {
            Id = documentId,
            LastPollTime = pollTime.ToString("O", CultureInfo.InvariantCulture),
            AuthorityId = authorityId,
        };

        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.PollState,
            doc,
            documentId,
            CosmosJsonSerializerContext.Default.PollStateDocument,
            ct).ConfigureAwait(false);
    }

    public async Task DeleteGlobalPollStateAsync(CancellationToken ct)
    {
        await this.client.DeleteDocumentAsync(
            CosmosContainerNames.PollState,
            GlobalDocumentId,
            GlobalDocumentId,
            ct).ConfigureAwait(false);
    }

    public async Task<IReadOnlyList<int>> GetLeastRecentlyPolledAsync(
        IReadOnlyList<int> candidateAuthorityIds,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(candidateAuthorityIds);

        if (candidateAuthorityIds.Count == 0)
        {
            return [];
        }

        var docs = await this.client.QueryAsync(
            CosmosContainerNames.PollState,
            "SELECT * FROM c WHERE c.authorityId != null",
            parameters: null,
            partitionKey: null,
            CosmosJsonSerializerContext.Default.PollStateDocument,
            ct).ConfigureAwait(false);

        var polledSet = docs.ToDictionary(d => d.AuthorityId, d => d.LastPollTime);

        // Never-polled authorities first, then by oldest lastPollTime
        return candidateAuthorityIds
            .OrderBy(id => polledSet.ContainsKey(id) ? 1 : 0)
            .ThenBy(id => polledSet.TryGetValue(id, out var time) ? time : string.Empty)
            .ToList();
    }

    private static string FormatDocumentId(int authorityId)
    {
        return string.Create(CultureInfo.InvariantCulture, $"poll-state-{authorityId}");
    }
}
