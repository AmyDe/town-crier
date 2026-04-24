using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePollingLeaseStore : IPollingLeaseStore
{
    private LeaseHandle? held;
    private int acquireCalls;
    private int releaseCalls;

    public int AcquireCalls => this.acquireCalls;

    public int ReleaseCalls => this.releaseCalls;

    /// <summary>
    /// Gets or sets a value indicating whether every acquire returns Held until cleared.
    /// </summary>
    public bool SimulateHeld { get; set; }

    /// <summary>
    /// Gets or sets the exception to return on the next acquire call as a transient error.
    /// </summary>
    public Exception? NextAcquireException { get; set; }

    public Task<LeaseAcquireResult> TryAcquireAsync(TimeSpan ttl, CancellationToken ct)
    {
        Interlocked.Increment(ref this.acquireCalls);

        if (this.NextAcquireException is { } ex)
        {
            this.NextAcquireException = null;
            return Task.FromResult(LeaseAcquireResult.FromTransient(ex));
        }

        if (this.SimulateHeld || this.held is not null)
        {
            return Task.FromResult(LeaseAcquireResult.FromHeld());
        }

        this.held = new LeaseHandle($"\"etag-{Guid.NewGuid():N}\"");
        return Task.FromResult(LeaseAcquireResult.FromAcquired(this.held));
    }

    public Task<LeaseReleaseOutcome> ReleaseAsync(LeaseHandle handle, CancellationToken ct)
    {
        Interlocked.Increment(ref this.releaseCalls);
        if (this.held is not null && this.held.ETag == handle.ETag)
        {
            this.held = null;
            return Task.FromResult(LeaseReleaseOutcome.Released);
        }

        return Task.FromResult(LeaseReleaseOutcome.AlreadyGone);
    }
}
