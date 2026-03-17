namespace TownCrier.Application.UserProfiles;

public sealed record UpdateZonePreferencesCommand(
    string UserId,
    string ZoneId,
    bool NewApplications,
    bool StatusChanges,
    bool DecisionUpdates);
