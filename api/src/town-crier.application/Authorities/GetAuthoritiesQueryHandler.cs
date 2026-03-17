namespace TownCrier.Application.Authorities;

public sealed class GetAuthoritiesQueryHandler
{
    private readonly IAuthorityProvider authorityProvider;

    public GetAuthoritiesQueryHandler(IAuthorityProvider authorityProvider)
    {
        this.authorityProvider = authorityProvider;
    }

    public async Task<GetAuthoritiesResult> HandleAsync(GetAuthoritiesQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var all = await this.authorityProvider.GetAllAsync(ct).ConfigureAwait(false);

        IEnumerable<TownCrier.Domain.Authorities.Authority> filtered = all;

        if (!string.IsNullOrWhiteSpace(query.Search))
        {
            filtered = filtered.Where(a =>
                a.Name.Contains(query.Search, StringComparison.OrdinalIgnoreCase));
        }

        var sorted = filtered
            .OrderBy(a => a.Name, StringComparer.OrdinalIgnoreCase)
            .Select(a => new AuthorityListItem(a.Id, a.Name, a.AreaType))
            .ToList();

        return new GetAuthoritiesResult(sorted, sorted.Count);
    }
}
