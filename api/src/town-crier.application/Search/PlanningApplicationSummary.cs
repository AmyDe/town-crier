using System.Diagnostics.CodeAnalysis;

namespace TownCrier.Application.Search;

[SuppressMessage("Design", "CA1056:URI properties should not be strings", Justification = "PlanIt API returns URLs as strings")]
[SuppressMessage("Design", "CA1054:URI parameters should not be strings", Justification = "PlanIt API returns URLs as strings")]
public sealed record PlanningApplicationSummary(
    string Uid,
    string Name,
    string Address,
    string? Postcode,
    string Description,
    string AppType,
    string AppState,
    string AreaName,
    DateOnly? StartDate,
    string? Url);
