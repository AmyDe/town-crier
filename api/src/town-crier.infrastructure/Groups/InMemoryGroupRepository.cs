using System.Collections.Concurrent;
using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;

namespace TownCrier.Infrastructure.Groups;

public sealed class InMemoryGroupRepository : IGroupRepository
{
    private readonly ConcurrentDictionary<string, Group> store = new();

    public Task<Group?> GetByIdAsync(string groupId, CancellationToken ct)
    {
        this.store.TryGetValue(groupId, out var group);
        return Task.FromResult(group);
    }

    public Task SaveAsync(Group group, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(group);
        this.store[group.Id] = group;
        return Task.CompletedTask;
    }

    public Task DeleteAsync(string groupId, CancellationToken ct)
    {
        this.store.TryRemove(groupId, out _);
        return Task.CompletedTask;
    }

    public Task<IReadOnlyList<Group>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var groups = this.store.Values
            .Where(g => g.IsMember(userId))
            .ToList();
        return Task.FromResult<IReadOnlyList<Group>>(groups);
    }
}
