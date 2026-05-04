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
}
