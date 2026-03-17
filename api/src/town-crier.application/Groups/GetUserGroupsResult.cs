namespace TownCrier.Application.Groups;

public sealed record GetUserGroupsResult(IReadOnlyList<UserGroupSummary> Groups);
