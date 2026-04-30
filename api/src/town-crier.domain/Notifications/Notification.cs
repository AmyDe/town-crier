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
        string? decision = null)
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
            pushSent,
            emailSent,
            createdAt);
    }
}
