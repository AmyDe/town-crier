using TownCrier.Domain.Groups;

namespace TownCrier.Application.Groups;

public sealed class DeleteGroupCommandHandler
{
    private readonly IGroupRepository groupRepository;

    public DeleteGroupCommandHandler(IGroupRepository groupRepository)
    {
        this.groupRepository = groupRepository;
    }

    public async Task HandleAsync(DeleteGroupCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var group = await this.groupRepository.GetByIdAsync(command.GroupId, ct).ConfigureAwait(false)
            ?? throw new GroupNotFoundException();

        if (group.OwnerId != command.RequestingUserId)
        {
            throw new UnauthorizedGroupOperationException("Only the group owner can delete the group.");
        }

        await this.groupRepository.DeleteAsync(command.GroupId, ct).ConfigureAwait(false);
    }
}
