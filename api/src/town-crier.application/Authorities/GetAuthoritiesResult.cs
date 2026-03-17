namespace TownCrier.Application.Authorities;

public sealed record GetAuthoritiesResult(IReadOnlyList<AuthorityListItem> Authorities, int Total);
