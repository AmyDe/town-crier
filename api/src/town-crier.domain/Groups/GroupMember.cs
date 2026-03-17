namespace TownCrier.Domain.Groups;

public sealed class GroupMember
{
    private GroupMember(string userId, GroupRole role, DateTimeOffset joinedAt)
    {
        this.UserId = userId;
        this.Role = role;
        this.JoinedAt = joinedAt;
    }

    public string UserId { get; }

    public GroupRole Role { get; }

    public DateTimeOffset JoinedAt { get; }

    internal static GroupMember CreateOwner(string userId, DateTimeOffset now)
    {
        return new GroupMember(userId, GroupRole.Owner, now);
    }

    internal static GroupMember CreateMember(string userId, DateTimeOffset now)
    {
        return new GroupMember(userId, GroupRole.Member, now);
    }
}
