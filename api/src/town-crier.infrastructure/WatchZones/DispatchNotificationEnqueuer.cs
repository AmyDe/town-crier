using TownCrier.Application.Notifications;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Infrastructure.WatchZones;

public sealed class DispatchNotificationEnqueuer : INotificationEnqueuer
{
    private readonly DispatchNotificationCommandHandler handler;

    public DispatchNotificationEnqueuer(DispatchNotificationCommandHandler handler)
    {
        this.handler = handler;
    }

    public async Task EnqueueAsync(PlanningApplication application, WatchZone matchedZone, CancellationToken ct)
    {
        var command = new DispatchNotificationCommand(application, matchedZone);
        await this.handler.HandleAsync(command, ct).ConfigureAwait(false);
    }
}
