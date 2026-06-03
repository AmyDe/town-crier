using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Notifications;

public interface INotificationRepository
{
    /// <summary>
    /// Returns the existing notification (if any) for the (userId, applicationUid,
    /// authorityId, eventType) tuple. Authority must be part of the key because
    /// PlanIt uids are only unique within a council — without it, a Bradford
    /// decision would suppress a Kingston one for the same uid (bd tc-th98 / GH#384).
    /// </summary>
    /// <param name="userId">The Auth0 sub for the owning user.</param>
    /// <param name="applicationUid">The PlanIt-assigned application uid.</param>
    /// <param name="authorityId">The PlanIt areaId for the council that issued the uid.</param>
    /// <param name="eventType">The notification event type (NewApplication or DecisionUpdate).</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>The existing notification for the (userId, applicationUid, authorityId, eventType) tuple, or null if none.</returns>
    Task<Notification?> GetByUserAndApplicationAsync(
        string userId,
        string applicationUid,
        int authorityId,
        NotificationEventType eventType,
        CancellationToken ct);

    Task<int> CountByUserSinceAsync(string userId, DateTimeOffset since, CancellationToken ct);

    /// <summary>
    /// Counts the user's notifications whose <c>CreatedAt</c> is strictly greater
    /// than <paramref name="lastReadAt"/> — i.e. unread per the watermark model.
    /// Drives the iOS app icon badge value computed at push-send time.
    /// </summary>
    /// <param name="userId">The Auth0 sub for the owning user.</param>
    /// <param name="lastReadAt">The user's notification-state watermark. Boundary is exclusive.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>The count of notifications strictly after the watermark.</returns>
    Task<int> GetUnreadCountAsync(string userId, DateTimeOffset lastReadAt, CancellationToken ct);

    /// <summary>
    /// For a set of application uids, returns the most-recent unread notification per
    /// uid — i.e. the latest event under the watermark model whose <c>CreatedAt</c> is
    /// strictly greater than <paramref name="lastReadAt"/> — in a single round-trip. A
    /// uid with no qualifying notification is absent from the result map. Drives the
    /// <c>latestUnreadEvent</c> field on each row of the applications-by-zone result;
    /// collapses the per-application N+1 loop into O(1) Cosmos queries (bd tc-1wkp).
    /// </summary>
    /// <param name="userId">The Auth0 sub for the owning user (partition key).</param>
    /// <param name="applicationUids">The PlanIt application uids to look up.</param>
    /// <param name="lastReadAt">The user's notification-state watermark. Boundary is exclusive.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>A map from application uid to its most-recent unread notification.</returns>
    Task<IReadOnlyDictionary<string, Notification>> GetLatestUnreadByApplicationsAsync(
        string userId,
        IReadOnlyCollection<string> applicationUids,
        DateTimeOffset lastReadAt,
        CancellationToken ct);

    Task<IReadOnlyList<Notification>> GetByUserSinceAsync(string userId, DateTimeOffset since, CancellationToken ct);

    Task<IReadOnlyList<Notification>> GetUnsentEmailsByUserAsync(string userId, CancellationToken ct);

    // Cross-partition — worker path only (GenerateHourlyDigestsCommandHandler).
    Task<IReadOnlyList<string>> GetUserIdsWithUnsentEmailsCrossPartitionAsync(CancellationToken ct);

    Task SaveAsync(Notification notification, CancellationToken ct);

    Task DeleteAllByUserIdAsync(string userId, CancellationToken ct);
}
