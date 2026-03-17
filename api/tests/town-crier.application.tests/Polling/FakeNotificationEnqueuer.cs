using TownCrier.Application.WatchZones;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakeNotificationEnqueuer : INotificationEnqueuer
{
    private readonly List<(PlanningApplication Application, WatchZone Zone)> enqueued = [];

    public IReadOnlyCollection<(PlanningApplication Application, WatchZone Zone)> Enqueued => this.enqueued;

    public Task EnqueueAsync(PlanningApplication application, WatchZone matchedZone, CancellationToken ct)
    {
        this.enqueued.Add((application, matchedZone));
        return Task.CompletedTask;
    }
}
