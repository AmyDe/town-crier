using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.PlanningApplications;

public interface IPlanningApplicationRepository
{
    Task UpsertAsync(PlanningApplication application, CancellationToken ct);
}
