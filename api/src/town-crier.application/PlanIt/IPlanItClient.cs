using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.PlanIt;

public interface IPlanItClient
{
    // Streams applications for the given authority. maxPages bounds pagination
    // to that many pages per call (null = unbounded / natural end-of-data exit).
    // Seed-poll cycles pass a cap to prevent a backlogged authority from
    // monopolising the rate budget before rotation advances. See bd tc-l77h.
    IAsyncEnumerable<PlanningApplication> FetchApplicationsAsync(
        int authorityId,
        DateTimeOffset? differentStart,
        int? maxPages,
        CancellationToken ct);

    Task<PlanItSearchResult> SearchApplicationsAsync(
        string searchText,
        int authorityId,
        int page,
        CancellationToken ct);
}
