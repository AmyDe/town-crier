namespace TownCrier.Domain.UserProfiles;

public sealed class UserProfile
{
    private UserProfile(
        string userId,
        string? postcode,
        NotificationPreferences notificationPreferences,
        SubscriptionTier tier)
    {
        this.UserId = userId;
        this.Postcode = postcode;
        this.NotificationPreferences = notificationPreferences;
        this.Tier = tier;
    }

    public string UserId { get; }

    public string? Postcode { get; private set; }

    public NotificationPreferences NotificationPreferences { get; private set; }

    public SubscriptionTier Tier { get; }

    public static UserProfile Register(string userId)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        return new UserProfile(
            userId,
            postcode: null,
            notificationPreferences: NotificationPreferences.Default,
            tier: SubscriptionTier.Free);
    }

    public void UpdatePreferences(string? postcode, NotificationPreferences notificationPreferences)
    {
        this.Postcode = postcode;
        this.NotificationPreferences = notificationPreferences;
    }
}
