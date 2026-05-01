using TownCrier.Domain.Notifications;

namespace TownCrier.Infrastructure.Tests.Notifications;

internal sealed class NotificationBuilder
{
    private string userId = "user-1";
    private string applicationUid = "test-uid-001";
    private string applicationName = "APP/2026/0001";
    private string watchZoneId = "zone-1";
    private string applicationAddress = "123 High Street";
    private string applicationDescription = "Single storey rear extension";
    private string applicationType = "Householder";
    private int authorityId = 42;
    private DateTimeOffset createdAt = new(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);
    private string? decision;
    private NotificationEventType eventType = NotificationEventType.NewApplication;
    private NotificationSources sources = NotificationSources.Zone;

    public NotificationBuilder WithUserId(string userId)
    {
        this.userId = userId;
        return this;
    }

    public NotificationBuilder WithApplicationUid(string applicationUid)
    {
        this.applicationUid = applicationUid;
        return this;
    }

    public NotificationBuilder WithApplicationName(string applicationName)
    {
        this.applicationName = applicationName;
        return this;
    }

    public NotificationBuilder WithWatchZoneId(string watchZoneId)
    {
        this.watchZoneId = watchZoneId;
        return this;
    }

    public NotificationBuilder WithApplicationAddress(string address)
    {
        this.applicationAddress = address;
        return this;
    }

    public NotificationBuilder WithApplicationDescription(string description)
    {
        this.applicationDescription = description;
        return this;
    }

    public NotificationBuilder WithApplicationType(string type)
    {
        this.applicationType = type;
        return this;
    }

    public NotificationBuilder WithAuthorityId(int authorityId)
    {
        this.authorityId = authorityId;
        return this;
    }

    public NotificationBuilder WithCreatedAt(DateTimeOffset createdAt)
    {
        this.createdAt = createdAt;
        return this;
    }

    public NotificationBuilder WithDecision(string? decision)
    {
        this.decision = decision;
        return this;
    }

    public NotificationBuilder WithEventType(NotificationEventType eventType)
    {
        this.eventType = eventType;
        return this;
    }

    public NotificationBuilder WithSources(NotificationSources sources)
    {
        this.sources = sources;
        return this;
    }

    public NotificationBuilder WithWatchZoneIdOrNull(string? watchZoneId)
    {
        this.watchZoneId = watchZoneId!;
        return this;
    }

    public Notification Build()
    {
        return Notification.Create(
            userId: this.userId,
            applicationUid: this.applicationUid,
            applicationName: this.applicationName,
            watchZoneId: this.watchZoneId,
            applicationAddress: this.applicationAddress,
            applicationDescription: this.applicationDescription,
            applicationType: this.applicationType,
            authorityId: this.authorityId,
            now: this.createdAt,
            decision: this.decision,
            eventType: this.eventType,
            sources: this.sources);
    }
}
