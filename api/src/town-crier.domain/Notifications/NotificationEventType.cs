namespace TownCrier.Domain.Notifications;

/// <summary>
/// Distinguishes the lifecycle event a Notification was raised for. New
/// applications and decision updates are persisted side-by-side and routed
/// to the same dispatch and digest pipelines, so we tag them at the entity
/// level rather than splitting the storage container.
/// </summary>
public enum NotificationEventType
{
    /// <summary>
    /// A planning application appeared in PlanIt for the first time and
    /// matched at least one of the user's subscriptions.
    /// </summary>
    NewApplication = 0,

    /// <summary>
    /// A previously-tracked application transitioned into a decision state
    /// (Permitted, Conditions, Rejected, Appealed).
    /// </summary>
    DecisionUpdate = 1,
}
