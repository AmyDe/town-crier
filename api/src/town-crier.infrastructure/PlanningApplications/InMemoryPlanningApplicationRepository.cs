using System.Collections.Concurrent;
using TownCrier.Application.PlanningApplications;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Infrastructure.PlanningApplications;

public sealed class InMemoryPlanningApplicationRepository : IPlanningApplicationRepository
{
    private readonly ConcurrentDictionary<string, PlanningApplication> store = new();

    public Task UpsertAsync(PlanningApplication application, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(application);
        this.store[application.Name] = application;
        return Task.CompletedTask;
    }
}
