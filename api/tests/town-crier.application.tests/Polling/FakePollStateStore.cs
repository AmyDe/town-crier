using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePollStateStore : IPollStateStore
{
    private readonly Dictionary<int, DateTimeOffset> pollTimes = [];

    public int SaveCallCount { get; private set; }

    public DateTimeOffset? GetLastPollTimeFor(int authorityId)
    {
        return this.pollTimes.TryGetValue(authorityId, out var time) ? time : null;
    }

    public void SetLastPollTime(int authorityId, DateTimeOffset pollTime)
    {
        this.pollTimes[authorityId] = pollTime;
    }

    public Task<DateTimeOffset?> GetLastPollTimeAsync(int authorityId, CancellationToken ct)
    {
        DateTimeOffset? result = this.pollTimes.TryGetValue(authorityId, out var time) ? time : null;
        return Task.FromResult(result);
    }

    public Task SaveLastPollTimeAsync(int authorityId, DateTimeOffset pollTime, CancellationToken ct)
    {
        this.pollTimes[authorityId] = pollTime;
        this.SaveCallCount++;
        return Task.CompletedTask;
    }

    public Task<IReadOnlyList<int>> GetLeastRecentlyPolledAsync(
        IReadOnlyList<int> candidateAuthorityIds,
        CancellationToken ct)
    {
        IReadOnlyList<int> sorted = candidateAuthorityIds
            .OrderBy(id => this.pollTimes.ContainsKey(id) ? 1 : 0)
            .ThenBy(id => this.pollTimes.TryGetValue(id, out var time) ? time : DateTimeOffset.MinValue)
            .ToList();
        return Task.FromResult(sorted);
    }
}
