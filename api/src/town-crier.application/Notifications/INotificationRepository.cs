using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Notifications;

public interface INotificationRepository
{
    Task<Notification?> GetByUserAndApplicationAsync(
        string userId,
        string applicationUid,
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
    /// Returns the most-recent notification for the given user and application
    /// whose <c>CreatedAt</c> is strictly greater than <paramref name="lastReadAt"/> —
    /// i.e. the latest unread event under the watermark model. Returns <c>null</c>
    /// when no qualifying notification exists. Drives the <c>latestUnreadEvent</c>
    /// field on each row of the applications-by-zone result.
    /// </summary>
    /// <param name="userId">The Auth0 sub for the owning user.</param>
    /// <param name="applicationUid">The PlanIt-assigned application uid.</param>
    /// <param name="lastReadAt">The user's notification-state watermark. Boundary is exclusive.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>The most-recent unread notification, or <c>null</c> if none.</returns>
    Task<Notification?> GetLatestUnreadByApplicationAsync(
        string userId,
        string applicationUid,
        DateTimeOffset lastReadAt,
        CancellationToken ct);

    Task<IReadOnlyList<Notification>> GetByUserSinceAsync(string userId, DateTimeOffset since, CancellationToken ct);

    Task<IReadOnlyList<Notification>> GetUnsentEmailsByUserAsync(string userId, CancellationToken ct);

    Task<IReadOnlyList<string>> GetUserIdsWithUnsentEmailsAsync(CancellationToken ct);

    Task SaveAsync(Notification notification, CancellationToken ct);

    Task DeleteAllByUserIdAsync(string userId, CancellationToken ct);
}
