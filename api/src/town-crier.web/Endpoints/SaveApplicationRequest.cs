using System.Diagnostics.CodeAnalysis;

namespace TownCrier.Web.Endpoints;

// Body of PUT /v1/me/saved-applications/{applicationUid}. Carries the full
// PlanningApplication payload so the API can upsert it into Cosmos at save time
// instead of relying on the search hot loop's upsert. See bead tc-if12.
[SuppressMessage("Design", "CA1054:URI parameters should not be strings", Justification = "PlanIt API returns URLs as strings")]
[SuppressMessage("Design", "CA1056:URI properties should not be strings", Justification = "PlanIt API returns URLs as strings")]
internal sealed record SaveApplicationRequest(
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
