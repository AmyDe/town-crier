namespace TownCrier.Application.UserProfiles;

public sealed record GetZonePreferencesResult(
    string ZoneId,
    bool NewApplications,
    bool StatusChanges,
    bool DecisionUpdates);
