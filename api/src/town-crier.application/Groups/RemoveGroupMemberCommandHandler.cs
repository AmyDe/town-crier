namespace TownCrier.Application.Groups;

public sealed class RemoveGroupMemberCommandHandler
{
    private readonly IGroupRepository groupRepository;

    public RemoveGroupMemberCommandHandler(IGroupRepository groupRepository)
    {
        this.groupRepository = groupRepository;
    }

    public async Task HandleAsync(RemoveGroupMemberCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var group = await this.groupRepository.GetByIdAsync(command.GroupId, ct).ConfigureAwait(false)
            ?? throw new GroupNotFoundException();

        group.RemoveMember(command.RequestingUserId, command.MemberUserId);

        await this.groupRepository.SaveAsync(group, ct).ConfigureAwait(false);
    }
}
