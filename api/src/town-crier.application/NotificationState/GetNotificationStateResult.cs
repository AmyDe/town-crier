namespace TownCrier.Application.NotificationState;

/// <summary>
/// Wire shape for <c>GET /v1/me/notification-state</c>. Surfaces the watermark
/// (so clients can replay it on advance), the version (so clients can detect
/// out-of-band mutations across devices), and the totalUnreadCount (drives the
/// app icon badge and the Applications-screen Unread chip). See
/// <c>docs/specs/notifications-unread-watermark.md#api-endpoints</c>.
/// </summary>
/// <param name="LastReadAt">The current watermark — notifications strictly newer than this are unread.</param>
/// <param name="Version">The aggregate's mutation counter; bumps on every successful mark/advance.</param>
/// <param name="TotalUnreadCount">The count of notifications strictly after <paramref name="LastReadAt"/>.</param>
public sealed record GetNotificationStateResult(
    DateTimeOffset LastReadAt,
    int Version,
    int TotalUnreadCount);
