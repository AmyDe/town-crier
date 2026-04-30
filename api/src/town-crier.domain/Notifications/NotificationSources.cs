namespace TownCrier.Domain.Notifications;

/// <summary>
/// Identifies which subscription paths produced a Notification. A single
/// underlying application can match a user via multiple sources (e.g. it
/// falls inside a watch zone <em>and</em> the user has saved it). The
/// dispatch handler OR-merges those matches into a single Notification with
/// the relevant flags set, rather than emitting duplicates.
/// </summary>
[Flags]
public enum NotificationSources
{
    /// <summary>
    /// No subscription source — only valid as the identity element when
    /// constructing a flag set.
    /// </summary>
    None = 0,

    /// <summary>
    /// The application matched a user's watch zone (geographic radius).
    /// </summary>
    Zone = 1,

    /// <summary>
    /// The application is in the user's Saved list (manually bookmarked).
    /// </summary>
    Saved = 2,
}
