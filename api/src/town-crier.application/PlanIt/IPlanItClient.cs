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

    // Fetches a single application by UID via PlanIt's per-application endpoint
    // (/planapplic/{uid}/json). Returns null when PlanIt has no record (404).
    // Used by GetApplicationByUidQueryHandler as a fallback when Cosmos has
    // never seen the application.
    Task<PlanningApplication?> GetByUidAsync(string uid, CancellationToken ct);
}
