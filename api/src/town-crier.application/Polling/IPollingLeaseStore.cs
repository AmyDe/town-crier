namespace TownCrier.Application.Polling;

/// <summary>
/// Best-effort distributed lease for the polling cycle. Backed by the existing
/// Cosmos <c>Leases</c> container. Used by <see cref="PollPlanItCommandHandler"/>
/// to prevent concurrent poll cycles when the Service Bus-triggered run and the
/// safety-net cron run overlap.
/// </summary>
public interface IPollingLeaseStore
{
    /// <summary>
    /// Attempts to acquire the polling lease. Returns <c>false</c> when another
    /// holder's TTL is still live — the caller must then exit cleanly without
    /// polling and without publishing a follow-up Service Bus message.
    /// </summary>
    /// <param name="ttl">How long the lease should remain valid before auto-expiry.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns><c>true</c> if the lease was acquired; <c>false</c> if another holder is still live.</returns>
    Task<bool> TryAcquireAsync(TimeSpan ttl, CancellationToken ct);

    /// <summary>
    /// Releases the lease held by the current process. Idempotent — safe to call
    /// from a finally block even when the acquire failed.
    /// </summary>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>A task that completes when the release has been confirmed.</returns>
    Task ReleaseAsync(CancellationToken ct);
}
