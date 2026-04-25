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

        // Backward-compat: legacy documents (pre-tc-m6fx) stored the conflated
        // LastPollTime as the PlanIt cursor. When HighWaterMark is absent, fall
        // back to LastPollTime — that preserves cursor behaviour until the next
        // write populates both fields. See docs/specs/poll-state-split-last-poll-time.md.
        var highWaterMark = doc.HighWaterMark is null
            ? lastPollTime
            : DateTimeOffset.Parse(doc.HighWaterMark, CultureInfo.InvariantCulture);

        var cursor = ReadCursor(doc);
        return new PollState(lastPollTime, highWaterMark, cursor);
    }

    public async Task SaveAsync(
        int authorityId,
        DateTimeOffset lastPollTime,
        DateTimeOffset highWaterMark,
        PollCursor? cursor,
        CancellationToken ct)
    {
        var documentId = FormatDocumentId(authorityId);
        var doc = new PollStateDocument
        {
            Id = documentId,
            LastPollTime = lastPollTime.ToString("O", CultureInfo.InvariantCulture),
            HighWaterMark = highWaterMark.ToString("O", CultureInfo.InvariantCulture),
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

    public async Task<LeastRecentlyPolledResult> GetLeastRecentlyPolledAsync(
        IReadOnlyList<int> candidateAuthorityIds,
        CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(candidateAuthorityIds);

        if (candidateAuthorityIds.Count == 0)
        {
            return new LeastRecentlyPolledResult([], NeverPolledCount: 0);
        }

        var docs = await this.client.QueryAsync(
            CosmosContainerNames.PollState,
            "SELECT * FROM c WHERE STARTSWITH(c.id, 'poll-state-')",
            parameters: null,
            partitionKey: null,
            CosmosJsonSerializerContext.Default.PollStateDocument,
            ct).ConfigureAwait(false);

        var polledSet = docs.ToDictionary(d => d.AuthorityId, d => d.LastPollTime);

        // Never-polled authorities first, then by oldest lastPollTime (scheduling clock,
        // independent of HighWaterMark). See docs/specs/poll-state-split-last-poll-time.md.
        var sorted = candidateAuthorityIds
            .OrderBy(id => polledSet.ContainsKey(id) ? 1 : 0)
            .ThenBy(id => polledSet.TryGetValue(id, out var time) ? DateTimeOffset.Parse(time, CultureInfo.InvariantCulture) : DateTimeOffset.MinValue)
            .ToList();

        // Never-polled cohort = candidates with no PollState document. Surfaced via
        // the towncrier.polling.never_polled_count gauge so dashboards can detect
        // tc-ews7-style fairness regressions directly. See bd tc-ifdl.
        var neverPolled = candidateAuthorityIds.Count(id => !polledSet.ContainsKey(id));
        return new LeastRecentlyPolledResult(sorted, neverPolled);
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
