using TownCrier.Application.PlanIt;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePlanItClient : IPlanItClient
{
    private readonly List<PlanningApplication> applications = [];

    public DateTimeOffset? LastDifferentStartUsed { get; private set; }

    public void Add(PlanningApplication application)
    {
        this.applications.Add(application);
    }

    public async IAsyncEnumerable<PlanningApplication> FetchApplicationsAsync(
        DateTimeOffset? differentStart,
        [System.Runtime.CompilerServices.EnumeratorCancellation] CancellationToken ct)
    {
        this.LastDifferentStartUsed = differentStart;

        foreach (var app in this.applications)
        {
            yield return app;
        }

        await Task.CompletedTask.ConfigureAwait(false);
    }
}
