using TownCrier.Domain.Groups;

namespace TownCrier.Infrastructure.Groups;

internal sealed class GroupMemberDocument
{
    public required string UserId { get; init; }

    public required string Role { get; init; }

    public required DateTimeOffset JoinedAt { get; init; }

    public static GroupMemberDocument FromDomain(GroupMember member)
    {
        return new GroupMemberDocument
        {
            UserId = member.UserId,
            Role = member.Role.ToString(),
            JoinedAt = member.JoinedAt,
        };
    }

    public GroupMember ToDomain()
    {
        var role = Enum.Parse<GroupRole>(this.Role);
        return GroupMember.Reconstitute(this.UserId, role, this.JoinedAt);
    }
}
