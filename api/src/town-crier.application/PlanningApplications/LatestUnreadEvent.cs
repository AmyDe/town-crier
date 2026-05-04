using TownCrier.Domain.Notifications;

namespace TownCrier.Application.PlanningApplications;

/// <summary>
/// Per-application unread-notification descriptor surfaced on each row of the
/// applications-by-zone result. Drives the saturated colour and copy of the
/// status pill in iOS/web. <c>null</c> when no notification exists strictly
/// after the user's <c>lastReadAt</c> watermark, or the user has no watermark
/// document yet (first-touch path; clients will seed via
/// <c>GET /v1/me/notification-state</c>).
/// </summary>
/// <remarks>
/// See <c>docs/specs/notifications-unread-watermark.md#api-augment-applications</c>.
/// </remarks>
/// <param name="Type">The lifecycle event the notification was raised for.</param>
/// <param name="Decision">The raw PlanIt decision string (e.g. "Permitted") for <see cref="NotificationEventType.DecisionUpdate"/>; null otherwise.</param>
/// <param name="CreatedAt">The instant the notification was raised. Used by the iOS push-tap path to advance the watermark.</param>
public sealed record LatestUnreadEvent(
    NotificationEventType Type,
    string? Decision,
    DateTimeOffset CreatedAt);
