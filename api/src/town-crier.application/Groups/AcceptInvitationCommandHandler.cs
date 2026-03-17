namespace TownCrier.Application.Groups;

public sealed class AcceptInvitationCommandHandler
{
    private readonly IGroupRepository groupRepository;
    private readonly IGroupInvitationRepository invitationRepository;
    private readonly TimeProvider timeProvider;

    public AcceptInvitationCommandHandler(
        IGroupRepository groupRepository,
        IGroupInvitationRepository invitationRepository,
        TimeProvider timeProvider)
    {
        this.groupRepository = groupRepository;
        this.invitationRepository = invitationRepository;
        this.timeProvider = timeProvider;
    }

    public async Task HandleAsync(AcceptInvitationCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var invitation = await this.invitationRepository.GetByIdAsync(command.InvitationId, ct).ConfigureAwait(false)
            ?? throw new InvalidOperationException("Invitation not found.");

        var now = this.timeProvider.GetUtcNow();
        invitation.Accept(now);

        var group = await this.groupRepository.GetByIdAsync(invitation.GroupId, ct).ConfigureAwait(false)
            ?? throw new GroupNotFoundException();

        group.AddMember(command.UserId, now);

        await this.groupRepository.SaveAsync(group, ct).ConfigureAwait(false);
        await this.invitationRepository.SaveAsync(invitation, ct).ConfigureAwait(false);
    }
}
