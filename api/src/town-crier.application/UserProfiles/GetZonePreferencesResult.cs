namespace TownCrier.Application.UserProfiles;

public sealed record GetZonePreferencesResult(
    string ZoneId,
    bool NewApplicationPush,
    bool NewApplicationEmail,
    bool DecisionPush,
    bool DecisionEmail);
