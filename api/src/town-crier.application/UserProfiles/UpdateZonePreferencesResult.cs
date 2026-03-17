namespace TownCrier.Application.UserProfiles;

public sealed record UpdateZonePreferencesResult(
    string ZoneId,
    bool NewApplications,
    bool StatusChanges,
    bool DecisionUpdates);
