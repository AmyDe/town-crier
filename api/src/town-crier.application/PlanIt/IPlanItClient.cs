using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.PlanIt;

public interface IPlanItClient
{
    IAsyncEnumerable<PlanningApplication> FetchApplicationsAsync(
        int authorityId,
        DateTimeOffset? differentStart,
        CancellationToken ct);
}
