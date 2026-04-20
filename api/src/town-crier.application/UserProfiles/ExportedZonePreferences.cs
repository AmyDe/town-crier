namespace TownCrier.Application.UserProfiles;

public sealed record ExportedZonePreferences(
    string ZoneId,
    bool NewApplications,
    bool StatusChanges,
    bool DecisionUpdates);
