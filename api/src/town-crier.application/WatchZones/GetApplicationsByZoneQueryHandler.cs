using System.Globalization;
using TownCrier.Application.PlanningApplications;

namespace TownCrier.Application.WatchZones;

public sealed class GetApplicationsByZoneQueryHandler
{
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly IPlanningApplicationRepository applicationRepository;

    public GetApplicationsByZoneQueryHandler(
        IWatchZoneRepository watchZoneRepository,
        IPlanningApplicationRepository applicationRepository)
    {
        this.watchZoneRepository = watchZoneRepository;
        this.applicationRepository = applicationRepository;
    }

    public async Task<IReadOnlyList<PlanningApplicationResult>?> HandleAsync(
        GetApplicationsByZoneQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var zone = await this.watchZoneRepository.GetByUserAndZoneIdAsync(
            query.UserId, query.ZoneId, ct).ConfigureAwait(false);

        if (zone is null)
        {
            return null;
        }

        var authorityCode = zone.AuthorityId.ToString(CultureInfo.InvariantCulture);
        var applications = await this.applicationRepository.FindNearbyAsync(
            authorityCode,
            zone.Centre.Latitude,
            zone.Centre.Longitude,
            zone.RadiusMetres,
            ct).ConfigureAwait(false);

        return applications.Select(GetApplicationByUidQueryHandler.ToResult).ToList();
    }
}
