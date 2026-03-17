namespace TownCrier.Application.Groups;

public sealed record InviteMemberCommand(
    string RequestingUserId,
    string GroupId,
    string InvitationId,
    string InviteeEmail);
