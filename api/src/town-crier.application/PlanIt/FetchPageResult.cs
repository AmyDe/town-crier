using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.PlanIt;

/// <summary>
/// Single page of results returned by
/// <see cref="IPlanItClient.FetchApplicationsPageAsync"/>.
/// </summary>
/// <param name="PageNumber">The page number this result corresponds to (1-based).</param>
/// <param name="Applications">Applications returned on this page.</param>
/// <param name="Total">
/// Total number of matching applications reported by PlanIt for the query, if
/// present in the response. Typically only populated on the first page.
/// </param>
/// <param name="HasMorePages">
/// True when more pages are expected after this one. Derived by the client
/// from the page fill ratio (<c>Applications.Count &gt;= DefaultPageSize</c>).
/// </param>
public sealed record FetchPageResult(
    int PageNumber,
    IReadOnlyList<PlanningApplication> Applications,
    int? Total,
    bool HasMorePages);
