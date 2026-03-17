namespace TownCrier.Application.Groups;

public sealed record DeleteGroupCommand(string RequestingUserId, string GroupId);
