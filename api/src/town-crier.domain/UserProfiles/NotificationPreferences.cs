namespace TownCrier.Domain.UserProfiles;

public sealed record NotificationPreferences(bool PushEnabled, DayOfWeek DigestDay = DayOfWeek.Monday)
{
    public static NotificationPreferences Default => new(PushEnabled: true);
}
