namespace TownCrier.Domain.Notifications;

public sealed class Notification
{
    private Notification(
        string id,
        string userId,
        string applicationName,
        string watchZoneId,
        string applicationAddress,
        string applicationDescription,
        string applicationType,
        int authorityId,
        bool pushSent,
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
        this.PushSent = pushSent;
        this.CreatedAt = createdAt;
    }

    public string Id { get; }

    public string UserId { get; }

    public string ApplicationName { get; }

    public string WatchZoneId { get; }

    public string ApplicationAddress { get; }

    public string ApplicationDescription { get; }

    public string ApplicationType { get; }

    public int AuthorityId { get; }

    public bool PushSent { get; private set; }

    public DateTimeOffset CreatedAt { get; }

    public static Notification Create(
        string userId,
        string applicationName,
        string watchZoneId,
        string applicationAddress,
        string applicationDescription,
        string applicationType,
        int authorityId,
        DateTimeOffset now)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        ArgumentException.ThrowIfNullOrWhiteSpace(applicationName);
        ArgumentException.ThrowIfNullOrWhiteSpace(watchZoneId);

        return new Notification(
            id: Guid.NewGuid().ToString(),
            userId: userId,
            applicationName: applicationName,
            watchZoneId: watchZoneId,
            applicationAddress: applicationAddress,
            applicationDescription: applicationDescription,
            applicationType: applicationType,
            authorityId: authorityId,
            pushSent: false,
            createdAt: now);
    }

    public void MarkPushSent()
    {
        this.PushSent = true;
    }
}
