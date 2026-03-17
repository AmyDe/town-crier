using System.Collections.Concurrent;
using TownCrier.Application.Polling;

namespace TownCrier.Infrastructure.Polling;

public sealed class InMemoryActiveAuthorityProvider : IActiveAuthorityProvider
{
    private readonly ConcurrentDictionary<int, byte> authorityIds = new();

    public void Add(int authorityId)
    {
        this.authorityIds.TryAdd(authorityId, 0);
    }

    public void Remove(int authorityId)
    {
        this.authorityIds.TryRemove(authorityId, out _);
    }

    public Task<IReadOnlyCollection<int>> GetActiveAuthorityIdsAsync(CancellationToken ct)
    {
        return Task.FromResult<IReadOnlyCollection<int>>(this.authorityIds.Keys.ToList().AsReadOnly());
    }
}
