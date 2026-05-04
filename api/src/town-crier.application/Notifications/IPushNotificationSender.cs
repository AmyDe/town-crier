using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Notifications;

public interface IPushNotificationSender
{
    /// <summary>
    /// Sends an alert push for a single notification.
    /// </summary>
    /// <param name="notification">The notification record being delivered.</param>
    /// <param name="devices">Target device registrations for the user.</param>
    /// <param name="totalUnreadCount">
    /// The user's total unread-notification count to surface as the iOS app icon
    /// badge. Computed from the user's notification-state watermark plus any
    /// just-created notification not yet persisted. Drives the badge so taps
    /// followed by mark-all-read can decrement back to zero on subsequent pushes.
    /// </param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>The send outcome, including any tokens APNs reported as invalid.</returns>
    Task<PushSendResult> SendAsync(
        Notification notification,
        IReadOnlyList<DeviceRegistration> devices,
        int totalUnreadCount,
        CancellationToken ct);

    /// <summary>
    /// Sends a digest push summarising recent activity.
    /// </summary>
    /// <param name="applicationCount">Number of new applications captured in the digest body copy.</param>
    /// <param name="totalUnreadCount">
    /// The user's total unread-notification count to surface as the iOS app icon
    /// badge. Distinct from <paramref name="applicationCount"/>: the digest body
    /// counts the applications in the digest window; the badge counts everything
    /// the user hasn't yet read across all time.
    /// </param>
    /// <param name="devices">Target device registrations for the user.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>The send outcome, including any tokens APNs reported as invalid.</returns>
    Task<PushSendResult> SendDigestAsync(
        int applicationCount,
        int totalUnreadCount,
        IReadOnlyList<DeviceRegistration> devices,
        CancellationToken ct);
}
