using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePollStateStore : IPollStateStore
{
    public DateTimeOffset? LastPollTime { get; private set; }

    public int SaveCallCount { get; private set; }

    public void SetLastPollTime(DateTimeOffset pollTime)
    {
        this.LastPollTime = pollTime;
    }

    public Task<DateTimeOffset?> GetLastPollTimeAsync(CancellationToken ct)
    {
        return Task.FromResult(this.LastPollTime);
    }

    public Task SaveLastPollTimeAsync(DateTimeOffset pollTime, CancellationToken ct)
    {
        this.LastPollTime = pollTime;
        this.SaveCallCount++;
        return Task.CompletedTask;
    }
}
