namespace TownCrier.Domain.UserProfiles;

public sealed record NotificationPreferences(
    bool PushEnabled,
    DayOfWeek DigestDay = DayOfWeek.Monday,
    bool EmailDigestEnabled = true,
    bool EmailInstantEnabled = false)
{
    public static NotificationPreferences Default => new(PushEnabled: true);
}
