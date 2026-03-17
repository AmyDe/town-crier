namespace TownCrier.Application.Groups;

public sealed class GetGroupQueryHandler
{
    private readonly IGroupRepository groupRepository;

    public GetGroupQueryHandler(IGroupRepository groupRepository)
    {
        this.groupRepository = groupRepository;
    }

    public async Task<GetGroupResult> HandleAsync(GetGroupQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var group = await this.groupRepository.GetByIdAsync(query.GroupId, ct).ConfigureAwait(false)
            ?? throw new GroupNotFoundException();

        if (!group.IsMember(query.UserId))
        {
            throw new GroupNotFoundException();
        }

        var members = group.Members
            .Select(m => new GroupMemberResult(m.UserId, m.Role.ToString(), m.JoinedAt))
            .ToList();

        return new GetGroupResult(
            group.Id,
            group.Name,
            group.OwnerId,
            group.Centre.Latitude,
            group.Centre.Longitude,
            group.RadiusMetres,
            group.AuthorityId,
            members);
    }
}
