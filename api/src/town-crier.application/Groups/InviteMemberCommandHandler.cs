using TownCrier.Domain.Groups;

namespace TownCrier.Application.Groups;

public sealed class InviteMemberCommandHandler
{
    private static readonly TimeSpan InvitationValidity = TimeSpan.FromDays(7);

    private readonly IGroupRepository groupRepository;
    private readonly IGroupInvitationRepository invitationRepository;
    private readonly TimeProvider timeProvider;

    public InviteMemberCommandHandler(
        IGroupRepository groupRepository,
        IGroupInvitationRepository invitationRepository,
        TimeProvider timeProvider)
    {
        this.groupRepository = groupRepository;
        this.invitationRepository = invitationRepository;
        this.timeProvider = timeProvider;
    }

    public async Task<InviteMemberResult> HandleAsync(InviteMemberCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var group = await this.groupRepository.GetByIdAsync(command.GroupId, ct).ConfigureAwait(false)
            ?? throw new GroupNotFoundException();

        if (group.OwnerId != command.RequestingUserId)
        {
            throw new UnauthorizedGroupOperationException("Only the group owner can invite members.");
        }

        var now = this.timeProvider.GetUtcNow();

        var invitation = GroupInvitation.Create(
            command.InvitationId,
            command.GroupId,
            command.InviteeEmail,
            command.RequestingUserId,
            now,
            InvitationValidity);

        await this.invitationRepository.SaveAsync(invitation, ct).ConfigureAwait(false);

        return new InviteMemberResult(invitation.Id, invitation.GroupId, invitation.InviteeEmail);
    }
}
