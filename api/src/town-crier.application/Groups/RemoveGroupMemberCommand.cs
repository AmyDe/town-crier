namespace TownCrier.Application.Groups;

public sealed record RemoveGroupMemberCommand(
    string RequestingUserId,
    string GroupId,
    string MemberUserId);
