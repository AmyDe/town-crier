using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;

namespace TownCrier.Application.Tests.Groups;

internal sealed class FakeGroupRepository : IGroupRepository
{
    private readonly List<Group> store = [];

    public int Count => this.store.Count;

    public Task<Group?> GetByIdAsync(string groupId, CancellationToken ct)
    {
        var group = this.store.Find(g => g.Id == groupId);
        return Task.FromResult(group);
    }

    public Task SaveAsync(Group group, CancellationToken ct)
    {
        var existing = this.store.FindIndex(g => g.Id == group.Id);
        if (existing >= 0)
        {
            this.store[existing] = group;
        }
        else
        {
            this.store.Add(group);
        }

        return Task.CompletedTask;
    }

    public Task DeleteAsync(string groupId, CancellationToken ct)
    {
        this.store.RemoveAll(g => g.Id == groupId);
        return Task.CompletedTask;
    }

    public Task<IReadOnlyList<Group>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var groups = this.store
            .Where(g => g.Members.Any(m => m.UserId == userId))
            .ToList();
        return Task.FromResult<IReadOnlyList<Group>>(groups);
    }
}
