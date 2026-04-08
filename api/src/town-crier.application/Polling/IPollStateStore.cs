namespace TownCrier.Application.Polling;

public interface IPollStateStore
{
    Task<DateTimeOffset?> GetLastPollTimeAsync(int authorityId, CancellationToken ct);

    Task SaveLastPollTimeAsync(int authorityId, DateTimeOffset pollTime, CancellationToken ct);

    Task DeleteGlobalPollStateAsync(CancellationToken ct);

    Task<IReadOnlyList<int>> GetLeastRecentlyPolledAsync(
        IReadOnlyList<int> candidateAuthorityIds,
        CancellationToken ct);
}
