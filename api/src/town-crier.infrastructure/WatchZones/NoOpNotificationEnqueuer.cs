using TownCrier.Application.WatchZones;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Infrastructure.WatchZones;

public sealed class NoOpNotificationEnqueuer : INotificationEnqueuer
{
    public Task EnqueueAsync(PlanningApplication application, WatchZone matchedZone, CancellationToken ct)
    {
        return Task.CompletedTask;
    }
}
