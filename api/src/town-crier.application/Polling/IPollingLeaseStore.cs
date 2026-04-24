namespace TownCrier.Application.Polling;

/// <summary>
/// ETag-CAS distributed lease for the polling cycle. Both
/// <see cref="PollTriggerOrchestrator"/> and <see cref="PollTriggerBootstrapper"/>
/// acquire this lease before any action that could mutate the poll queue.
/// Serialisation is enforced via Cosmos If-Match / If-None-Match preconditions.
/// </summary>
public interface IPollingLeaseStore
{
    /// <summary>
    /// Attempts to acquire the polling lease with the given TTL. Returns an
    /// <see cref="LeaseAcquireResult"/> distinguishing Acquired / Held /
    /// TransientError. Never throws for expected outcomes (held by peer,
    /// raced on create/replace).
    /// </summary>
    Task<LeaseAcquireResult> TryAcquireAsync(TimeSpan ttl, CancellationToken ct);

    /// <summary>
    /// Releases the lease identified by <paramref name="handle"/>. Performs a
    /// conditional delete using the ETag from acquire. Never throws — failures
    /// are logged via the store's own logger, if any.
    /// </summary>
    Task ReleaseAsync(LeaseHandle handle, CancellationToken ct);
}
