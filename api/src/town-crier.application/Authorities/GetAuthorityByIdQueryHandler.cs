namespace TownCrier.Application.Authorities;

public sealed class GetAuthorityByIdQueryHandler
{
    private readonly IAuthorityProvider authorityProvider;

    public GetAuthorityByIdQueryHandler(IAuthorityProvider authorityProvider)
    {
        this.authorityProvider = authorityProvider;
    }

    public async Task<GetAuthorityByIdResult?> HandleAsync(GetAuthorityByIdQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var authority = await this.authorityProvider.GetByIdAsync(query.AuthorityId, ct).ConfigureAwait(false);
        if (authority is null)
        {
            return null;
        }

        return new GetAuthorityByIdResult(
            authority.Id,
            authority.Name,
            authority.AreaType,
            authority.CouncilUrl,
            authority.PlanningUrl);
    }
}
