#pragma warning disable CA1054, CA1056
namespace TownCrier.Application.Authorities;

public sealed record GetAuthorityByIdResult(
    int Id,
    string Name,
    string AreaType,
    string? CouncilUrl,
    string? PlanningUrl);
#pragma warning restore CA1054, CA1056
