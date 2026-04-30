namespace TownCrier.Application.UserProfiles;

public sealed record ExportedZonePreferences(
    string ZoneId,
    bool NewApplicationPush,
    bool NewApplicationEmail,
    bool DecisionPush,
    bool DecisionEmail);
