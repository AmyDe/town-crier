using System.Globalization;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Polling;

public sealed class CosmosPollStateStore : IPollStateStore
{
    private readonly ICosmosRestClient client;

    public CosmosPollStateStore(ICosmosRestClient client)
    {
        ArgumentNullException.ThrowIfNull(client);
        this.client = client;
    }

    public async Task<PollState?> GetAsync(int authorityId, CancellationToken ct)
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

        var lastPollTime = DateTimeOffset.Parse(doc.LastPollTime, CultureInfo.InvariantCulture);
        var cursor = ReadCursor(doc);
        return new PollState(lastPollTime, cursor);
    }

    public async Task SaveAsync(
        int authorityId,
        DateTimeOffset lastPollTime,
        PollCursor? cursor,
        CancellationToken ct)
    {
        var documentId = FormatDocumentId(authorityId);
        var doc = new PollStateDocument
        {
            Id = documentId,
            LastPollTime = lastPollTime.ToString("O", CultureInfo.InvariantCulture),
            AuthorityId = authorityId,
            CursorDifferentStart = cursor?.DifferentStart.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture),
            CursorNextPage = cursor?.NextPage,
            CursorKnownTotal = cursor?.KnownTotal,
        };

        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.PollState,
            doc,
            documentId,
            CosmosJsonSerializerContext.Default.PollStateDocument,
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
            "SELECT * FROM c WHERE STARTSWITH(c.id, 'poll-state-')",
            parameters: null,
            partitionKey: null,
            CosmosJsonSerializerContext.Default.PollStateDocument,
            ct).ConfigureAwait(false);

        var polledSet = docs.ToDictionary(d => d.AuthorityId, d => d.LastPollTime);

        // Never-polled authorities first, then by oldest lastPollTime
        return candidateAuthorityIds
            .OrderBy(id => polledSet.ContainsKey(id) ? 1 : 0)
            .ThenBy(id => polledSet.TryGetValue(id, out var time) ? DateTimeOffset.Parse(time, CultureInfo.InvariantCulture) : DateTimeOffset.MinValue)
            .ToList();
    }

    private static PollCursor? ReadCursor(PollStateDocument doc)
    {
        // All three cursor fields move as a set — if any is absent, there is no active cursor.
        if (doc.CursorDifferentStart is null || doc.CursorNextPage is null)
        {
            return null;
        }

        var differentStart = DateOnly.ParseExact(
            doc.CursorDifferentStart,
            "yyyy-MM-dd",
            CultureInfo.InvariantCulture);
        return new PollCursor(differentStart, doc.CursorNextPage.Value, doc.CursorKnownTotal);
    }

    private static string FormatDocumentId(int authorityId)
    {
        return string.Create(CultureInfo.InvariantCulture, $"poll-state-{authorityId}");
    }
}
