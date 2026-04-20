namespace TownCrier.Application.UserProfiles;

public sealed record ExportedDecisionAlert(
    string Id,
    string ApplicationUid,
    string ApplicationName,
    string ApplicationAddress,
    string Decision,
    bool PushSent,
    DateTimeOffset CreatedAt);
