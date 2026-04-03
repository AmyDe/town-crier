using TownCrier.Application.Authorities;

namespace TownCrier.Application.PlanningApplications;

public sealed record GetUserApplicationAuthoritiesResult(
    IReadOnlyList<AuthorityListItem> Authorities,
    int Count);
