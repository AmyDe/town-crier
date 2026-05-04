namespace TownCrier.Application.NotificationState;

/// <summary>
/// Returns the caller's notification watermark and the count of notifications
/// strictly newer than it. First-touch users get a fresh watermark seeded at
/// the current instant — see spec Pre-Resolved Decision #13 ("clean slate").
/// </summary>
/// <param name="UserId">The Auth0 sub of the caller.</param>
public sealed record GetNotificationStateQuery(string UserId);
