using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.PlanIt;

public interface IPlanItClient
{
    IAsyncEnumerable<PlanningApplication> FetchApplicationsAsync(
        DateTimeOffset? differentStart,
        CancellationToken ct);
}
