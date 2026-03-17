namespace TownCrier.Domain.SavedApplications;

public sealed class SavedApplication
{
    private SavedApplication(string userId, string applicationUid, DateTimeOffset savedAt)
    {
        this.UserId = userId;
        this.ApplicationUid = applicationUid;
        this.SavedAt = savedAt;
    }

    public string UserId { get; }

    public string ApplicationUid { get; }

    public DateTimeOffset SavedAt { get; }

    public static SavedApplication Create(string userId, string applicationUid, DateTimeOffset now)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);
        ArgumentException.ThrowIfNullOrWhiteSpace(applicationUid);
        return new SavedApplication(userId, applicationUid, now);
    }
}
