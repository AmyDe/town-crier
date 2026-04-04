using TownCrier.Application.Observability;

namespace TownCrier.Application.WatchZones;

public sealed class DeleteWatchZoneCommandHandler
{
    private readonly IWatchZoneRepository watchZoneRepository;

    public DeleteWatchZoneCommandHandler(IWatchZoneRepository watchZoneRepository)
    {
        this.watchZoneRepository = watchZoneRepository;
    }

    public async Task HandleAsync(DeleteWatchZoneCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        await this.watchZoneRepository.DeleteAsync(command.UserId, command.ZoneId, ct).ConfigureAwait(false);
        ApiMetrics.WatchZonesDeleted.Add(1);
    }
}
