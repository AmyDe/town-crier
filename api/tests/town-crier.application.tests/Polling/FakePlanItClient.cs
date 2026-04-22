using TownCrier.Application.PlanIt;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePlanItClient : IPlanItClient
{
    // Simulated PlanIt page size. Chosen small so tests can easily trigger the
    // page-fill heuristic (HasMorePages) with a handful of applications.
    public const int PageSize = 100;

    private readonly Dictionary<int, List<PlanningApplication>> applicationsByAuthority = [];
    private readonly Dictionary<int, Exception> exceptionsByAuthority = [];
    private readonly Dictionary<int, (int Count, Exception Exception)> throwAfterYieldingByAuthority = [];
    private readonly List<PlanningApplication> searchResults = [];
    private readonly Dictionary<int, int> yieldedByAuthority = [];

    public DateTimeOffset? LastDifferentStartUsed { get; private set; }

    public Dictionary<int, DateTimeOffset?> DifferentStartByAuthority { get; } = [];

    public List<int> AuthorityIdsRequested { get; } = [];

    public List<(int AuthorityId, int Page)> PagesRequested { get; } = [];

    public Exception? ExceptionToThrow { get; set; }

    /// <summary>
    /// Gets or sets a delay applied to each page fetch (honoring cancellation)
    /// before returning results or throwing.
    /// </summary>
    public TimeSpan? FetchDelay { get; set; }

    /// <summary>
    /// Gets or sets an optional override for the <see cref="FetchPageResult.Total"/>
    /// returned by <see cref="FetchApplicationsPageAsync"/>. When null the fake derives
    /// the total from the number of pre-seeded applications for the authority.
    /// </summary>
    public int? TotalOverride { get; set; }

    public int SearchTotal { get; set; }

    public string? LastSearchText { get; private set; }

    public int? LastAuthorityId { get; private set; }

    /// <summary>
    /// Gets or sets a callback invoked after each page fetch has assembled its
    /// result but before <see cref="FetchApplicationsPageAsync"/> returns. Lets
    /// tests advance a fake time provider to simulate wall-clock progression
    /// across pages (used by handler-budget tests).
    /// </summary>
    public Action<int, int>? OnFetchComplete { get; set; }

    public void AddSearchResult(PlanningApplication application)
    {
        this.searchResults.Add(application);
    }

    public void ThrowForAuthority(int authorityId, Exception exception)
    {
        this.exceptionsByAuthority[authorityId] = exception;
    }

    public void ThrowAfterYielding(int authorityId, int count, Exception exception)
    {
        this.throwAfterYieldingByAuthority[authorityId] = (count, exception);
    }

    public void Add(int authorityId, PlanningApplication application)
    {
        if (!this.applicationsByAuthority.TryGetValue(authorityId, out var list))
        {
            list = [];
            this.applicationsByAuthority[authorityId] = list;
        }

        list.Add(application);
    }

    public void Clear()
    {
        this.applicationsByAuthority.Clear();
    }

    public async Task<FetchPageResult> FetchApplicationsPageAsync(
        int authorityId,
        DateTimeOffset? differentStart,
        int page,
        CancellationToken ct)
    {
        this.LastDifferentStartUsed = differentStart;
        this.DifferentStartByAuthority[authorityId] = differentStart;
        this.AuthorityIdsRequested.Add(authorityId);
        this.PagesRequested.Add((authorityId, page));

        if (this.FetchDelay.HasValue)
        {
            await Task.Delay(this.FetchDelay.Value, ct).ConfigureAwait(false);
        }

        if (this.exceptionsByAuthority.TryGetValue(authorityId, out var authorityException))
        {
            throw authorityException;
        }

        if (this.ExceptionToThrow is not null)
        {
            throw this.ExceptionToThrow;
        }

        this.applicationsByAuthority.TryGetValue(authorityId, out var allApps);
        allApps ??= [];

        var skip = (page - 1) * PageSize;
        var pageItems = allApps.Skip(skip).Take(PageSize).ToList();

        // Emulate the previous streaming-fake's "throw after N yielded across the authority"
        // semantics in a page-oriented way:
        //   - Fill the first page with up to rule.Count apps and flag HasMorePages=true.
        //   - Throw on the next page request (after the handler has already processed the partial page).
        // This preserves the partial-progress contract the handler relies on for 429 mid-pagination tests.
        if (this.throwAfterYieldingByAuthority.TryGetValue(authorityId, out var rule))
        {
            var yielded = this.yieldedByAuthority.TryGetValue(authorityId, out var prev) ? prev : 0;

            // Throw immediately on subsequent pages — the "rate limit" has now kicked in.
            if (yielded >= rule.Count)
            {
                throw rule.Exception;
            }

            var remaining = rule.Count - yielded;
            var trimmed = pageItems.Take(remaining).ToList();
            this.yieldedByAuthority[authorityId] = yielded + trimmed.Count;

            // HasMorePages=true so the handler loops back and hits the throw on the next call.
            var totalForRule = this.TotalOverride ?? allApps.Count;
            var ruleResult = new FetchPageResult(page, trimmed, totalForRule, HasMorePages: true);
            this.OnFetchComplete?.Invoke(authorityId, page);
            return ruleResult;
        }

        var existingYielded = this.yieldedByAuthority.TryGetValue(authorityId, out var prevCount) ? prevCount : 0;
        this.yieldedByAuthority[authorityId] = existingYielded + pageItems.Count;

        var hasMorePages = pageItems.Count >= PageSize;
        var total = this.TotalOverride ?? allApps.Count;
        var result = new FetchPageResult(page, pageItems, total, hasMorePages);
        this.OnFetchComplete?.Invoke(authorityId, page);
        return result;
    }

    public Task<PlanItSearchResult> SearchApplicationsAsync(
        string searchText,
        int authorityId,
        int page,
        CancellationToken ct)
    {
        this.LastSearchText = searchText;
        this.LastAuthorityId = authorityId;
        return Task.FromResult(new PlanItSearchResult(this.searchResults, this.SearchTotal));
    }
}
