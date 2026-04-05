using TownCrier.Application.PlanIt;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePlanItClient : IPlanItClient
{
    private readonly Dictionary<int, List<PlanningApplication>> applicationsByAuthority = [];
    private readonly Dictionary<int, Exception> exceptionsByAuthority = [];
    private readonly List<PlanningApplication> searchResults = [];

    public DateTimeOffset? LastDifferentStartUsed { get; private set; }

    public List<int> AuthorityIdsRequested { get; } = [];

    public Exception? ExceptionToThrow { get; set; }

    public int SearchTotal { get; set; }

    public string? LastSearchText { get; private set; }

    public int? LastAuthorityId { get; private set; }

    public void AddSearchResult(PlanningApplication application)
    {
        this.searchResults.Add(application);
    }

    public void ThrowForAuthority(int authorityId, Exception exception)
    {
        this.exceptionsByAuthority[authorityId] = exception;
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

    public async IAsyncEnumerable<PlanningApplication> FetchApplicationsAsync(
        int authorityId,
        DateTimeOffset? differentStart,
        [System.Runtime.CompilerServices.EnumeratorCancellation] CancellationToken ct)
    {
        this.LastDifferentStartUsed = differentStart;
        this.AuthorityIdsRequested.Add(authorityId);

        if (this.exceptionsByAuthority.TryGetValue(authorityId, out var authorityException))
        {
            throw authorityException;
        }

        if (this.ExceptionToThrow is not null)
        {
            throw this.ExceptionToThrow;
        }

        if (this.applicationsByAuthority.TryGetValue(authorityId, out var applications))
        {
            foreach (var app in applications)
            {
                yield return app;
            }
        }

        await Task.CompletedTask.ConfigureAwait(false);
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
