using TownCrier.Application.Authorities;
using TownCrier.Application.WatchZones;

namespace TownCrier.Application.PlanningApplications;

public sealed class GetUserApplicationAuthoritiesQueryHandler
{
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly IAuthorityProvider authorityProvider;

    public GetUserApplicationAuthoritiesQueryHandler(
        IWatchZoneRepository watchZoneRepository,
        IAuthorityProvider authorityProvider)
    {
        this.watchZoneRepository = watchZoneRepository;
        this.authorityProvider = authorityProvider;
    }

    public async Task<GetUserApplicationAuthoritiesResult> HandleAsync(
        GetUserApplicationAuthoritiesQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var zones = await this.watchZoneRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);

        var distinctAuthorityIds = zones
            .Select(z => z.AuthorityId)
            .Distinct()
            .ToHashSet();

        var authorities = new List<AuthorityListItem>();
        foreach (var authorityId in distinctAuthorityIds)
        {
            var authority = await this.authorityProvider.GetByIdAsync(authorityId, ct).ConfigureAwait(false);
            if (authority is not null)
            {
                authorities.Add(new AuthorityListItem(authority.Id, authority.Name, authority.AreaType));
            }
        }

        authorities.Sort((a, b) => string.Compare(a.Name, b.Name, StringComparison.OrdinalIgnoreCase));

        return new GetUserApplicationAuthoritiesResult(authorities, authorities.Count);
    }
}
