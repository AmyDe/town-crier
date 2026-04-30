namespace TownCrier.Domain.Notifications;

public sealed class Notification
{
    private Notification(
        string id,
        string userId,
        string applicationName,
        string? watchZoneId,
        string applicationAddress,
        string applicationDescription,
        string? applicationType,
        int authorityId,
        string? decision,
        NotificationEventType eventType,
        NotificationSources sources,
        bool pushSent,
        bool emailSent,
        DateTimeOffset createdAt)
    {
        this.Id = id;
        this.UserId = userId;
        this.ApplicationName = applicationName;
        this.WatchZoneId = watchZoneId;
        this.ApplicationAddress = applicationAddress;
        this.ApplicationDescription = applicationDescription;
        this.ApplicationType = applicationType;
        this.AuthorityId = authorityId;
        this.Decision = decision;
        this.EventType = eventType;
        this.Sources = sources;
        this.PushSent = pushSent;
        this.EmailSent = emailSent;
        this.CreatedAt = createdAt;
    }

    public string Id { get; }

    public string UserId { get; }

    public string ApplicationName { get; }

    public string? WatchZoneId { get; }

    public string ApplicationAddress { get; }

    public string ApplicationDescription { get; }

    public string? ApplicationType { get; }

    public int AuthorityId { get; }

    /// <summary>
    /// Gets the raw PlanIt application state when the notification represents a
    /// decision update (e.g. "Permitted", "Conditions", "Rejected",
    /// "Appealed"). Null for new-application notifications. Use
    /// <see cref="UkPlanningVocabulary.GetDisplayString(string?)"/> to render
    /// for display.
    /// </summary>
    public string? Decision { get; }

    /// <summary>
    /// Gets the lifecycle event this notification was raised for. Defaults to
    /// <see cref="NotificationEventType.NewApplication"/> for legacy rows
    /// persisted before this field existed.
    /// </summary>
    public NotificationEventType EventType { get; }

    /// <summary>
    /// Gets the subscription paths that produced this notification. A single
    /// underlying application can match a user via multiple sources, so the
    /// dispatch handler OR-merges those matches into one notification.
    /// </summary>
    public NotificationSources Sources { get; }

    public bool PushSent { get; private set; }

    public bool EmailSent { get; private set; }

    public DateTimeOffset CreatedAt { get; }

    public static Notification Create(
        string userId,
        string applicationName,
        string? watchZoneId,
        string applicationAddress,
        string applicationDescription,
        string? applicationType,
        int authorityId,
        DateTimeOffset now,
        string? decision = null,
        NotificationEventType eventType = NotificationEventType.NewApplication,
        NotificationSources sources = NotificationSources.Zone)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        ArgumentException.ThrowIfNullOrWhiteSpace(applicationName);

        return new Notification(
            id: Guid.NewGuid().ToString(),
            userId: userId,
            applicationName: applicationName,
            watchZoneId: watchZoneId,
            applicationAddress: applicationAddress,
            applicationDescription: applicationDescription,
            applicationType: applicationType,
            authorityId: authorityId,
            decision: decision,
            eventType: eventType,
            sources: sources,
            pushSent: false,
            emailSent: false,
            createdAt: now);
    }

    public void MarkPushSent()
    {
        this.PushSent = true;
    }

    public void MarkEmailSent()
    {
        this.EmailSent = true;
    }

    internal static Notification Reconstitute(
        string id,
        string userId,
        string applicationName,
        string? watchZoneId,
        string applicationAddress,
        string applicationDescription,
        string? applicationType,
        int authorityId,
        string? decision,
        NotificationEventType eventType,
        NotificationSources sources,
        bool pushSent,
        bool emailSent,
        DateTimeOffset createdAt)
    {
        return new Notification(
            id,
            userId,
            applicationName,
            watchZoneId,
            applicationAddress,
            applicationDescription,
            applicationType,
            authorityId,
            decision,
            eventType,
            sources,
            pushSent,
            emailSent,
            createdAt);
    }
}
