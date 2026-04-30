namespace TownCrier.Application.UserProfiles;

public sealed record UpdateZonePreferencesCommand(
    string UserId,
    string ZoneId,
    bool NewApplicationPush,
    bool NewApplicationEmail,
    bool DecisionPush,
    bool DecisionEmail);
