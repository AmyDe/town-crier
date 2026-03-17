using System.Runtime.CompilerServices;
using TownCrier.Application.PlanIt;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Tests.Search;

internal sealed class FakePlanItClient : IPlanItClient
{
    private readonly List<PlanningApplication> searchResults = [];

    public string? LastSearchText { get; private set; }

    public int? LastAuthorityId { get; private set; }

    public int? LastPage { get; private set; }

    public int SearchTotal { get; set; }

    public void AddSearchResult(PlanningApplication application)
    {
        this.searchResults.Add(application);
    }

    public Task<PlanItSearchResult> SearchApplicationsAsync(
        string searchText,
        int authorityId,
        int page,
        CancellationToken ct)
    {
        this.LastSearchText = searchText;
        this.LastAuthorityId = authorityId;
        this.LastPage = page;

        return Task.FromResult(new PlanItSearchResult(this.searchResults, this.SearchTotal));
    }

    public async IAsyncEnumerable<PlanningApplication> FetchApplicationsAsync(
        int authorityId,
        DateTimeOffset? differentStart,
        [EnumeratorCancellation] CancellationToken ct)
    {
        await Task.CompletedTask.ConfigureAwait(false);
        yield break;
    }
}
