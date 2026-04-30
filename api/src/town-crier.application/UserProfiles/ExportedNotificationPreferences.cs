namespace TownCrier.Application.UserProfiles;

public sealed record ExportedNotificationPreferences(
    bool PushEnabled,
    DayOfWeek DigestDay,
    bool EmailDigestEnabled,
    bool SavedDecisionPush,
    bool SavedDecisionEmail,
    IReadOnlyList<ExportedZonePreferences> ZonePreferences);
