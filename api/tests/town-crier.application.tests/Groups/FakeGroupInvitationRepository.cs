using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;

namespace TownCrier.Application.Tests.Groups;

internal sealed class FakeGroupInvitationRepository : IGroupInvitationRepository
{
    private readonly List<GroupInvitation> store = [];

    public int Count => this.store.Count;

    public Task<GroupInvitation?> GetByIdAsync(string invitationId, CancellationToken ct)
    {
        var invitation = this.store.Find(i => i.Id == invitationId);
        return Task.FromResult(invitation);
    }

    public Task SaveAsync(GroupInvitation invitation, CancellationToken ct)
    {
        var existing = this.store.FindIndex(i => i.Id == invitation.Id);
        if (existing >= 0)
        {
            this.store[existing] = invitation;
        }
        else
        {
            this.store.Add(invitation);
        }

        return Task.CompletedTask;
    }

    public Task<IReadOnlyList<GroupInvitation>> GetPendingByGroupIdAsync(string groupId, CancellationToken ct)
    {
        var results = this.store
            .Where(i => i.GroupId == groupId && i.Status == InvitationStatus.Pending)
            .ToList();
        return Task.FromResult<IReadOnlyList<GroupInvitation>>(results);
    }

    public Task<IReadOnlyList<GroupInvitation>> GetPendingByEmailAsync(string email, CancellationToken ct)
    {
#pragma warning disable CA1308 // Emails are normalized to lowercase per industry convention
        var normalizedEmail = email.Trim().ToLowerInvariant();
#pragma warning restore CA1308

        var results = this.store
            .Where(i => i.InviteeEmail.Equals(normalizedEmail, StringComparison.OrdinalIgnoreCase)
                && i.Status == InvitationStatus.Pending)
            .ToList();
        return Task.FromResult<IReadOnlyList<GroupInvitation>>(results);
    }
}
