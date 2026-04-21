using TownCrier.Application.Authorities;
using TownCrier.Application.Polling;

namespace TownCrier.Infrastructure.Authorities;

public sealed class AllAuthorityIdProvider : IAllAuthorityIdProvider
{
    // Regional aggregates and non-LPA containers that PlanIt exposes but which
    // never return planning applications. Polling them wastes RUs and skews
    // diagnostic queries (e.g. the oldest-HWM leaderboard). See bd tc-85b2.
    //
    // Note: "Crown Dependencies" (plural) is the aggregate bucket covering
    // Channel Islands; the individual "Crown Dependency" records (Jersey,
    // Guernsey, Isle of Man, etc.) remain pollable because they expose real
    // planning data through PlanIt.
    private static readonly HashSet<string> NonPollableAreaTypes = new(StringComparer.Ordinal)
    {
        "English Region",
        "UK Nation",
        "Cross Border Area",
        "Metropolitan County",
        "Crown Dependencies",
    };

    private readonly IAuthorityProvider authorityProvider;

    public AllAuthorityIdProvider(IAuthorityProvider authorityProvider)
    {
        this.authorityProvider = authorityProvider;
    }

    public async Task<IReadOnlyCollection<int>> GetActiveAuthorityIdsAsync(CancellationToken ct)
    {
        var authorities = await this.authorityProvider.GetAllAsync(ct).ConfigureAwait(false);
        return authorities
            .Where(a => !NonPollableAreaTypes.Contains(a.AreaType))
            .Select(a => a.Id)
            .ToList()
            .AsReadOnly();
    }
}
