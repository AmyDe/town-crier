using TownCrier.Application.PlanIt;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePlanItClient : IPlanItClient
{
    private readonly Dictionary<int, List<PlanningApplication>> applicationsByAuthority = [];

    public DateTimeOffset? LastDifferentStartUsed { get; private set; }

    public List<int> AuthorityIdsRequested { get; } = [];

    public Exception? ExceptionToThrow { get; set; }

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
}
