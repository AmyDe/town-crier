namespace TownCrier.Domain.NotificationState;

/// <summary>
/// Per-user watermark that drives the unread-notification model. A notification
/// is "unread" iff its <c>CreatedAt</c> is strictly greater than this aggregate's
/// <see cref="LastReadAt"/>. See <c>docs/specs/notifications-unread-watermark.md</c>.
/// </summary>
/// <remarks>
/// Named <c>NotificationStateAggregate</c> rather than <c>NotificationState</c>
/// to disambiguate from the containing namespace
/// <see cref="TownCrier.Domain.NotificationState"/>. The Cosmos document and
/// future endpoint DTOs follow the namespace's plain noun.
/// </remarks>
public sealed class NotificationStateAggregate
{
    private NotificationStateAggregate(string userId, DateTimeOffset lastReadAt, int version)
    {
        this.UserId = userId;
        this.LastReadAt = lastReadAt;
        this.Version = version;
    }

    /// <summary>
    /// Gets the Auth0 sub of the user this watermark belongs to. Doubles as the
    /// document id and the partition key on the <c>NotificationState</c> container.
    /// </summary>
    public string UserId { get; }

    /// <summary>
    /// Gets the cutoff timestamp. Notifications with <c>CreatedAt</c> strictly
    /// after this instant are considered unread.
    /// </summary>
    public DateTimeOffset LastReadAt { get; private set; }

    /// <summary>
    /// Gets the in-domain mutation counter. Incremented on every successful
    /// state transition (<see cref="MarkAllReadAt"/>, <see cref="AdvanceTo"/>).
    /// Persisted alongside the document so consumers can reason about ordering
    /// even if the Cosmos ETag is opaque to them.
    /// </summary>
    public int Version { get; private set; }

    /// <summary>
    /// Creates a fresh state for a user that has never marked anything read.
    /// Endpoint adapters seed this on first GET so the migration path is
    /// "clean slate at deploy time" per the spec's Pre-Resolved Decisions.
    /// </summary>
    /// <param name="userId">The Auth0 sub for the owning user. Must be non-empty.</param>
    /// <param name="lastReadAt">The initial watermark — typically the current time at first read.</param>
    /// <returns>A new aggregate with <see cref="Version"/> seeded to 1.</returns>
    public static NotificationStateAggregate Create(string userId, DateTimeOffset lastReadAt)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        return new NotificationStateAggregate(userId, lastReadAt, version: 1);
    }

    /// <summary>
    /// Rebuilds an aggregate from its persisted form. Use only from repository
    /// adapters — application code should go through <see cref="Create"/> or
    /// the mutator methods so version transitions are explicit.
    /// </summary>
    /// <param name="userId">The Auth0 sub for the owning user.</param>
    /// <param name="lastReadAt">The persisted watermark.</param>
    /// <param name="version">The persisted mutation counter.</param>
    /// <returns>An aggregate reflecting the stored state, with no version bump.</returns>
    public static NotificationStateAggregate Reconstitute(
        string userId, DateTimeOffset lastReadAt, int version)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        return new NotificationStateAggregate(userId, lastReadAt, version);
    }

    /// <summary>
    /// Sets the watermark to <paramref name="now"/> unconditionally. This is the
    /// user-driven Mark-All-Read action; the supplied instant is authoritative
    /// even if it lands behind the previous watermark (clock skew tolerated by
    /// design — distinct from <see cref="AdvanceTo"/> which is monotonic).
    /// Always increments <see cref="Version"/> so consumers can detect the change.
    /// </summary>
    /// <param name="now">The instant to record as the new watermark.</param>
    public void MarkAllReadAt(DateTimeOffset now)
    {
        this.LastReadAt = now;
        this.Version++;
    }

    /// <summary>
    /// Advances the watermark to <paramref name="asOf"/> only if it lies strictly
    /// after the current <see cref="LastReadAt"/>. Older or equal instants are
    /// no-ops (per spec Pre-Resolved Decision #11 — server never moves the
    /// watermark backwards). Used by the push-tap path so older notifications
    /// being opened can't churn the version counter.
    /// </summary>
    /// <param name="asOf">The candidate new watermark — typically the tapped notification's <c>CreatedAt</c>.</param>
    /// <returns><c>true</c> when the watermark moved forward; <c>false</c> when the call was a no-op.</returns>
    public bool AdvanceTo(DateTimeOffset asOf)
    {
        if (asOf <= this.LastReadAt)
        {
            return false;
        }

        this.LastReadAt = asOf;
        this.Version++;
        return true;
    }
}
