namespace TownCrier.Domain.UserProfiles;

public sealed record NotificationPreferences(
    bool PushEnabled,
    DayOfWeek DigestDay = DayOfWeek.Monday,
    bool EmailDigestEnabled = true,
    bool SavedDecisionPush = true,
    bool SavedDecisionEmail = true)
{
    public static NotificationPreferences Default => new(PushEnabled: true);
}
