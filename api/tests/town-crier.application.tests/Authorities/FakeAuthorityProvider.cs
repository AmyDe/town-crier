using TownCrier.Application.Authorities;
using TownCrier.Domain.Authorities;

namespace TownCrier.Application.Tests.Authorities;

internal sealed class FakeAuthorityProvider : IAuthorityProvider
{
    private readonly List<Authority> authorities = [];

    public void Add(Authority authority)
    {
        this.authorities.Add(authority);
    }

    public Task<IReadOnlyList<Authority>> GetAllAsync(CancellationToken ct)
    {
        return Task.FromResult<IReadOnlyList<Authority>>(this.authorities.AsReadOnly());
    }

    public Task<Authority?> GetByIdAsync(int id, CancellationToken ct)
    {
        var authority = this.authorities.Find(a => a.Id == id);
        return Task.FromResult(authority);
    }
}
