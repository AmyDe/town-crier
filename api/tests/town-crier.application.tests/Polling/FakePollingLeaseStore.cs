using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePollingLeaseStore : IPollingLeaseStore
{
    public bool AcquireResult { get; set; } = true;

    public int AcquireCalls { get; private set; }

    public int ReleaseCalls { get; private set; }

    public TimeSpan? LastRequestedTtl { get; private set; }

    public Task<bool> TryAcquireAsync(TimeSpan ttl, CancellationToken ct)
    {
        this.AcquireCalls++;
        this.LastRequestedTtl = ttl;
        return Task.FromResult(this.AcquireResult);
    }

    public Task ReleaseAsync(CancellationToken ct)
    {
        this.ReleaseCalls++;
        return Task.CompletedTask;
    }
}
