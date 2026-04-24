namespace TownCrier.Application.Polling;

/// <summary>
/// Outcome of a <see cref="IPollingLeaseStore.ReleaseAsync"/> call.
/// </summary>
public enum LeaseReleaseOutcome
{
    /// <summary>The lease document was deleted successfully.</summary>
    Released = 0,

    /// <summary>The document was not found — lease had already expired or been deleted.</summary>
    AlreadyGone = 1,

    /// <summary>The conditional delete failed because the ETag did not match — another writer replaced the document.</summary>
    PreconditionFailed = 2,

    /// <summary>A transient error (network, 5xx) occurred; TTL is the backstop.</summary>
    TransientError = 3,
}
