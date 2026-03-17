namespace TownCrier.Application.Groups;

public sealed class GetUserGroupsQueryHandler
{
    private readonly IGroupRepository groupRepository;

    public GetUserGroupsQueryHandler(IGroupRepository groupRepository)
    {
        this.groupRepository = groupRepository;
    }

    public async Task<GetUserGroupsResult> HandleAsync(GetUserGroupsQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var groups = await this.groupRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);

        var summaries = groups
            .Select(g =>
            {
                var member = g.Members.First(m => m.UserId == query.UserId);
                return new UserGroupSummary(g.Id, g.Name, member.Role.ToString(), g.Members.Count);
            })
            .ToList();

        return new GetUserGroupsResult(summaries);
    }
}
