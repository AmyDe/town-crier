namespace TownCrier.Application.Polling;

public interface IPollStateStore
{
    Task<PollState?> GetAsync(int authorityId, CancellationToken ct);

    Task SaveAsync(
        int authorityId,
        DateTimeOffset lastPollTime,
        DateTimeOffset highWaterMark,
        PollCursor? cursor,
        CancellationToken ct);

    Task<IReadOnlyList<int>> GetLeastRecentlyPolledAsync(
        IReadOnlyList<int> candidateAuthorityIds,
        CancellationToken ct);
}
