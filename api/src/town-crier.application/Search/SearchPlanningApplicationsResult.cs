namespace TownCrier.Application.Search;

public sealed record SearchPlanningApplicationsResult(
    IReadOnlyCollection<PlanningApplicationSummary> Applications,
    int Total,
    int Page);
