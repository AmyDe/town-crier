namespace TownCrier.Application.Groups;

public sealed record GroupMemberResult(string UserId, string Role, DateTimeOffset JoinedAt);
