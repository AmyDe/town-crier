using TownCrier.Domain.Groups;

namespace TownCrier.Application.Groups;

public interface IGroupInvitationRepository
{
    Task<GroupInvitation?> GetByIdAsync(string invitationId, CancellationToken ct);

    Task SaveAsync(GroupInvitation invitation, CancellationToken ct);

    Task<IReadOnlyList<GroupInvitation>> GetPendingByGroupIdAsync(string groupId, CancellationToken ct);

    Task<IReadOnlyList<GroupInvitation>> GetPendingByEmailAsync(string email, CancellationToken ct);
}
