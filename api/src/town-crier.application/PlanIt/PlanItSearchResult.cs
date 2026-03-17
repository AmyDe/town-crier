using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.PlanIt;

public sealed record PlanItSearchResult(
    IReadOnlyCollection<PlanningApplication> Applications,
    int Total);
