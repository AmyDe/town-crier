using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Notifications;

public interface IEmailSender
{
    /// <summary>
    /// Sends a hourly/weekly digest email composed of zone-grouped notifications
    /// and an optional Saved Applications section for bookmark-only decisions
    /// that don't fall inside any of the user's watch zones.
    /// </summary>
    Task SendDigestAsync(
        string userId,
        string email,
        IReadOnlyList<WatchZoneDigest> zoneSections,
        IReadOnlyList<Notification> savedApplications,
        CancellationToken ct);

    Task SendNotificationAsync(string userId, string email, Notification notification, CancellationToken ct);
}
