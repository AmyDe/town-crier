namespace TownCrier.Application.UserProfiles;

public sealed record ExportedNotification(
    string Id,
    string ApplicationName,
    string WatchZoneId,
    string ApplicationAddress,
    string ApplicationDescription,
    string? ApplicationType,
    int AuthorityId,
    bool PushSent,
    bool EmailSent,
    DateTimeOffset CreatedAt);
