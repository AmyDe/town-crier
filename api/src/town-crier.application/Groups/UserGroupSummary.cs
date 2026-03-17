namespace TownCrier.Application.Groups;

public sealed record UserGroupSummary(
    string GroupId,
    string Name,
    string Role,
    int MemberCount);
