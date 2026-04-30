namespace TownCrier.Domain.UserProfiles;

public sealed record ZoneNotificationPreferences(
    bool NewApplicationPush,
    bool NewApplicationEmail,
    bool DecisionPush,
    bool DecisionEmail)
{
    public static ZoneNotificationPreferences Default => new(
        NewApplicationPush: true,
        NewApplicationEmail: true,
        DecisionPush: true,
        DecisionEmail: true);
}
