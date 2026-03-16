using TownCrier.Application.PlanningApplications;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePlanningApplicationRepository : IPlanningApplicationRepository
{
    private readonly Dictionary<string, PlanningApplication> store = [];

    public IReadOnlyCollection<PlanningApplication> GetAll() => this.store.Values.ToList();

    public PlanningApplication? GetByName(string name)
    {
        this.store.TryGetValue(name, out var app);
        return app;
    }

    public Task UpsertAsync(PlanningApplication application, CancellationToken ct)
    {
        this.store[application.Name] = application;
        return Task.CompletedTask;
    }
}
