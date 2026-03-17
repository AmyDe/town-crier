using System.Collections.Concurrent;
using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;

namespace TownCrier.Infrastructure.Groups;

public sealed class InMemoryGroupInvitationRepository : IGroupInvitationRepository
{
    private readonly ConcurrentDictionary<string, GroupInvitation> store = new();

    public Task<GroupInvitation?> GetByIdAsync(string invitationId, CancellationToken ct)
    {
        this.store.TryGetValue(invitationId, out var invitation);
        return Task.FromResult(invitation);
    }

    public Task SaveAsync(GroupInvitation invitation, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(invitation);
        this.store[invitation.Id] = invitation;
        return Task.CompletedTask;
    }

    public Task<IReadOnlyList<GroupInvitation>> GetPendingByGroupIdAsync(string groupId, CancellationToken ct)
    {
        var results = this.store.Values
            .Where(i => i.GroupId == groupId && i.Status == InvitationStatus.Pending)
            .ToList();
        return Task.FromResult<IReadOnlyList<GroupInvitation>>(results);
    }

    public Task<IReadOnlyList<GroupInvitation>> GetPendingByEmailAsync(string email, CancellationToken ct)
    {
        var results = this.store.Values
            .Where(i => i.InviteeEmail == email && i.Status == InvitationStatus.Pending)
            .ToList();
        return Task.FromResult<IReadOnlyList<GroupInvitation>>(results);
    }
}
