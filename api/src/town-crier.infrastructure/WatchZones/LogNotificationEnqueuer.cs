using TownCrier.Application.WatchZones;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Infrastructure.WatchZones;

public sealed class LogNotificationEnqueuer : INotificationEnqueuer
{
    public Task EnqueueAsync(PlanningApplication application, WatchZone matchedZone, CancellationToken ct)
    {
        // Placeholder — will be replaced with a real notification queue (e.g. change feed, service bus)
        return Task.CompletedTask;
    }
}
