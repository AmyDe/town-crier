namespace TownCrier.Application.UserProfiles;

public sealed record UpdateZonePreferencesResult(
    string ZoneId,
    bool NewApplicationPush,
    bool NewApplicationEmail,
    bool DecisionPush,
    bool DecisionEmail);
