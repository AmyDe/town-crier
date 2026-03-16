namespace TownCrier.Domain.UserProfiles;

public sealed record NotificationPreferences(bool PushEnabled)
{
    public static NotificationPreferences Default => new(PushEnabled: true);
}
