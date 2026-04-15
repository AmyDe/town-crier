using TownCrier.Application.Observability;
using TownCrier.Domain.Geocoding;

namespace TownCrier.Application.WatchZones;

public sealed class UpdateWatchZoneCommandHandler
{
    private readonly IWatchZoneRepository watchZoneRepository;

    public UpdateWatchZoneCommandHandler(IWatchZoneRepository watchZoneRepository)
    {
        this.watchZoneRepository = watchZoneRepository;
    }

    public async Task<UpdateWatchZoneResult> HandleAsync(UpdateWatchZoneCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var zone = await this.watchZoneRepository.GetByUserAndZoneIdAsync(command.UserId, command.ZoneId, ct)
            .ConfigureAwait(false)
            ?? throw new WatchZoneNotFoundException();

        Coordinates? newCentre = null;
        if (command.Latitude.HasValue || command.Longitude.HasValue)
        {
            newCentre = new Coordinates(
                command.Latitude ?? zone.Centre.Latitude,
                command.Longitude ?? zone.Centre.Longitude);
        }

        var updated = zone.WithUpdates(
            name: command.Name,
            centre: newCentre,
            radiusMetres: command.RadiusMetres,
            authorityId: command.AuthorityId);

        await this.watchZoneRepository.SaveAsync(updated, ct).ConfigureAwait(false);
        ApiMetrics.WatchZonesUpdated.Add(1);

        return new UpdateWatchZoneResult(new WatchZoneSummary(
            updated.Id,
            updated.Name,
            updated.Centre.Latitude,
            updated.Centre.Longitude,
            updated.RadiusMetres,
            updated.AuthorityId));
    }
}
