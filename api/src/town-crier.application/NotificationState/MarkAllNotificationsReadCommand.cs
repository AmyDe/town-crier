namespace TownCrier.Application.NotificationState;

/// <summary>
/// Sets the caller's notification watermark to the current instant. The user's
/// existing notifications all become read (their <c>CreatedAt</c> &lt;= the new
/// watermark). Idempotent in effect — repeated calls re-stamp <c>now</c> and
/// bump the version. See <c>docs/specs/notifications-unread-watermark.md</c>.
/// </summary>
/// <param name="UserId">The Auth0 sub of the caller.</param>
public sealed record MarkAllNotificationsReadCommand(string UserId);
