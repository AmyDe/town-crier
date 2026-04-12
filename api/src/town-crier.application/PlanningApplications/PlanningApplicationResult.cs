#pragma warning disable CA1054, CA1056
namespace TownCrier.Application.PlanningApplications;

public sealed record PlanningApplicationResult(
    string Name,
    string Uid,
    string AreaName,
    int AreaId,
    string Address,
    string? Postcode,
    string Description,
    string? AppType,
    string? AppState,
    string? AppSize,
    DateOnly? StartDate,
    DateOnly? DecidedDate,
    DateOnly? ConsultedDate,
    double? Longitude,
    double? Latitude,
    string? Url,
    string? Link,
    DateTimeOffset LastDifferent);
#pragma warning restore CA1054, CA1056
