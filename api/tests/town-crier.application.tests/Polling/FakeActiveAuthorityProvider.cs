using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakeActiveAuthorityProvider : IActiveAuthorityProvider
{
    private readonly List<int> authorityIds = [];

    public void Add(int authorityId)
    {
        this.authorityIds.Add(authorityId);
    }

    public Task<IReadOnlyCollection<int>> GetActiveAuthorityIdsAsync(CancellationToken ct)
    {
        return Task.FromResult<IReadOnlyCollection<int>>(this.authorityIds.AsReadOnly());
    }
}
