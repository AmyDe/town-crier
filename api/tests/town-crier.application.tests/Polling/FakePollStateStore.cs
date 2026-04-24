using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePollStateStore : IPollStateStore
{
    private readonly Dictionary<int, PollState> states = [];

    public int SaveCallCount { get; private set; }

    /// <summary>
    /// Gets or sets an optional callback invoked on every SaveAsync.
    /// Tests can use this to trigger cancellation between authorities.
    /// </summary>
    public Action<int, DateTimeOffset>? OnSave { get; set; }

    public PollState? GetStateFor(int authorityId)
    {
        return this.states.TryGetValue(authorityId, out var state) ? state : null;
    }

    public DateTimeOffset? GetLastPollTimeFor(int authorityId)
    {
        return this.states.TryGetValue(authorityId, out var state) ? state.LastPollTime : null;
    }

    public DateTimeOffset? GetHighWaterMarkFor(int authorityId)
    {
        return this.states.TryGetValue(authorityId, out var state) ? state.HighWaterMark : null;
    }

    public PollCursor? GetCursorFor(int authorityId)
    {
        return this.states.TryGetValue(authorityId, out var state) ? state.Cursor : null;
    }

    public void SetState(int authorityId, PollState state)
    {
        this.states[authorityId] = state;
    }

    // Convenience setter used by existing tests that pre-date the split between
    // LastPollTime (scheduling) and HighWaterMark (PlanIt cursor). Seeds both
    // fields to the same value so legacy tests retain their original semantics.
    public void SetLastPollTime(int authorityId, DateTimeOffset pollTime)
    {
        this.states[authorityId] = new PollState(pollTime, pollTime, Cursor: null);
    }

    public Task<PollState?> GetAsync(int authorityId, CancellationToken ct)
    {
        PollState? result = this.states.TryGetValue(authorityId, out var state) ? state : null;
        return Task.FromResult(result);
    }

    public Task SaveAsync(
        int authorityId,
        DateTimeOffset lastPollTime,
        DateTimeOffset highWaterMark,
        PollCursor? cursor,
        CancellationToken ct)
    {
        this.states[authorityId] = new PollState(lastPollTime, highWaterMark, cursor);
        this.SaveCallCount++;
        this.OnSave?.Invoke(authorityId, lastPollTime);
        return Task.CompletedTask;
    }

    public Task<IReadOnlyList<int>> GetLeastRecentlyPolledAsync(
        IReadOnlyList<int> candidateAuthorityIds,
        CancellationToken ct)
    {
        IReadOnlyList<int> sorted = candidateAuthorityIds
            .OrderBy(id => this.states.ContainsKey(id) ? 1 : 0)
            .ThenBy(id => this.states.TryGetValue(id, out var state) ? state.LastPollTime : DateTimeOffset.MinValue)
            .ToList();
        return Task.FromResult(sorted);
    }
}
