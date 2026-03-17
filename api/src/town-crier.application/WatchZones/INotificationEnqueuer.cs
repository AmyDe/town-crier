using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.WatchZones;

public interface INotificationEnqueuer
{
    Task EnqueueAsync(PlanningApplication application, WatchZone matchedZone, CancellationToken ct);
}
