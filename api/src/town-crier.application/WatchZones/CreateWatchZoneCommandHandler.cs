using TownCrier.Application.Authorities;
using TownCrier.Application.Observability;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.Entitlements;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.WatchZones;

public sealed class CreateWatchZoneCommandHandler
{
    private readonly IAuthorityResolver authorityResolver;
    private readonly IPlanningApplicationRepository planningApplicationRepository;
    private readonly TimeProvider timeProvider;
    private readonly IUserProfileRepository userProfileRepository;
    private readonly IWatchZoneRepository watchZoneRepository;

    public CreateWatchZoneCommandHandler(
        IWatchZoneRepository watchZoneRepository,
        IUserProfileRepository userProfileRepository,
        IPlanningApplicationRepository planningApplicationRepository,
        IAuthorityResolver authorityResolver,
        TimeProvider timeProvider)
    {
        this.watchZoneRepository = watchZoneRepository;
        this.userProfileRepository = userProfileRepository;
        this.planningApplicationRepository = planningApplicationRepository;
        this.authorityResolver = authorityResolver;
        this.timeProvider = timeProvider;
    }

    public async Task<CreateWatchZoneResult> HandleAsync(CreateWatchZoneCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var profile = await this.userProfileRepository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false)
            ?? throw new InvalidOperationException($"User profile not found: {command.UserId}");

        var maxZones = EntitlementMap.LimitFor(profile.Tier, Quota.WatchZones);
        if (maxZones < int.MaxValue)
        {
            var existingZones = await this.watchZoneRepository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false);
            if (existingZones.Count >= maxZones)
            {
                throw new WatchZoneQuotaExceededException(maxZones);
            }
        }

        var authorityId = command.AuthorityId
            ?? await this.authorityResolver.ResolveFromCoordinatesAsync(command.Latitude, command.Longitude, ct).ConfigureAwait(false);

        var zone = new WatchZone(
            command.ZoneId,
            command.UserId,
            command.Name,
            new Coordinates(command.Latitude, command.Longitude),
            command.RadiusMetres,
            authorityId,
            this.timeProvider.GetUtcNow());

        await this.watchZoneRepository.SaveAsync(zone, ct).ConfigureAwait(false);
        ApiMetrics.WatchZonesCreated.Add(1);

        var authorityCode = authorityId.ToString(System.Globalization.CultureInfo.InvariantCulture);
        var nearbyApplications = await this.planningApplicationRepository.FindNearbyAsync(
            authorityCode, command.Latitude, command.Longitude, command.RadiusMetres, ct).ConfigureAwait(false);

        return new CreateWatchZoneResult(nearbyApplications);
    }
}
