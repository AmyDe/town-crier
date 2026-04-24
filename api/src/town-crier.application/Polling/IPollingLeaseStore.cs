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
    /// <param name="ttl">Lease time-to-live; the lease document is written with this Cosmos TTL.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>Acquired (with handle), Held (lost the race), or TransientError.</returns>
    Task<LeaseAcquireResult> TryAcquireAsync(TimeSpan ttl, CancellationToken ct);

    /// <summary>
    /// Releases the lease identified by <paramref name="handle"/>. Performs a
    /// conditional delete using the ETag from acquire. Never throws — failures
    /// are surfaced as a <see cref="LeaseReleaseOutcome"/> rather than exceptions.
    /// TTL is the backstop for any non-Released outcome.
    /// </summary>
    /// <param name="handle">Handle returned by a prior successful <see cref="TryAcquireAsync"/>.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>
    /// <see cref="LeaseReleaseOutcome.Released"/> on success;
    /// <see cref="LeaseReleaseOutcome.AlreadyGone"/> if the document was not found;
    /// <see cref="LeaseReleaseOutcome.PreconditionFailed"/> if the ETag did not match;
    /// <see cref="LeaseReleaseOutcome.TransientError"/> for network or 5xx failures.
    /// </returns>
    Task<LeaseReleaseOutcome> ReleaseAsync(LeaseHandle handle, CancellationToken ct);
}
