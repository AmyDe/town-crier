namespace TownCrier.Domain.UserProfiles;

public sealed record ZoneNotificationPreferences(
    bool NewApplications,
    bool StatusChanges,
    bool DecisionUpdates)
{
    public static ZoneNotificationPreferences Default => new(
        NewApplications: true,
        StatusChanges: false,
        DecisionUpdates: false);
}
