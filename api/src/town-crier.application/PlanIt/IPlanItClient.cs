using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.PlanIt;

public interface IPlanItClient
{
    // Fetches a single page of applications for the given authority. The caller
    // drives pagination (and any page-cap policy) by looping on the returned
    // <see cref="FetchPageResult.HasMorePages"/> flag. See
    // docs/specs/polling-resumable-cursor.md.
    Task<FetchPageResult> FetchApplicationsPageAsync(
        int authorityId,
        DateTimeOffset? differentStart,
        int page,
        CancellationToken ct);

    Task<PlanItSearchResult> SearchApplicationsAsync(
        string searchText,
        int authorityId,
        int page,
        CancellationToken ct);
}
