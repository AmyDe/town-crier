using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakeAllAuthorityIdProvider : IAllAuthorityIdProvider
{
    private readonly List<int> ids = [];

    public void Add(int id) => this.ids.Add(id);

    public Task<IReadOnlyCollection<int>> GetActiveAuthorityIdsAsync(CancellationToken ct)
        => Task.FromResult<IReadOnlyCollection<int>>(this.ids.AsReadOnly());
}
