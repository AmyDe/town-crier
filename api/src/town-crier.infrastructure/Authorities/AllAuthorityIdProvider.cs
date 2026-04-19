using TownCrier.Application.Authorities;
using TownCrier.Application.Polling;

namespace TownCrier.Infrastructure.Authorities;

public sealed class AllAuthorityIdProvider : IAllAuthorityIdProvider
{
    private readonly IAuthorityProvider authorityProvider;

    public AllAuthorityIdProvider(IAuthorityProvider authorityProvider)
    {
        this.authorityProvider = authorityProvider;
    }

    public async Task<IReadOnlyCollection<int>> GetActiveAuthorityIdsAsync(CancellationToken ct)
    {
        var authorities = await this.authorityProvider.GetAllAsync(ct).ConfigureAwait(false);
        return authorities.Select(a => a.Id).ToList().AsReadOnly();
    }
}
