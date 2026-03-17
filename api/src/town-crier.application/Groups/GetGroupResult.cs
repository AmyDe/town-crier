namespace TownCrier.Application.Groups;

public sealed record GetGroupResult(
    string GroupId,
    string Name,
    string OwnerId,
    double Latitude,
    double Longitude,
    double RadiusMetres,
    int AuthorityId,
    IReadOnlyList<GroupMemberResult> Members);
