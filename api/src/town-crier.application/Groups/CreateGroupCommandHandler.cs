using TownCrier.Domain.Geocoding;
using TownCrier.Domain.Groups;

namespace TownCrier.Application.Groups;

public sealed class CreateGroupCommandHandler
{
    private readonly IGroupRepository groupRepository;
    private readonly TimeProvider timeProvider;

    public CreateGroupCommandHandler(IGroupRepository groupRepository, TimeProvider timeProvider)
    {
        this.groupRepository = groupRepository;
        this.timeProvider = timeProvider;
    }

    public async Task<CreateGroupResult> HandleAsync(CreateGroupCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var group = Group.Create(
            command.GroupId,
            command.Name,
            command.UserId,
            new Coordinates(command.Latitude, command.Longitude),
            command.RadiusMetres,
            command.AuthorityId,
            this.timeProvider.GetUtcNow());

        await this.groupRepository.SaveAsync(group, ct).ConfigureAwait(false);

        return new CreateGroupResult(group.Id, group.Name);
    }
}
