namespace TownCrier.Application.UserProfiles;

public sealed record ExportedNotificationPreferences(
    bool PushEnabled,
    DayOfWeek DigestDay,
    bool EmailDigestEnabled,
    IReadOnlyList<ExportedZonePreferences> ZonePreferences);
