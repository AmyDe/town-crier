using TownCrier.Application.Authorities;
using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.UserProfiles;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.WatchZones;

public sealed class CreateWatchZoneCommandHandler
{
    private static readonly TimeSpan BackfillWindow = TimeSpan.FromDays(90);

    private readonly IAuthorityResolver authorityResolver;
    private readonly IPlanItClient planItClient;
    private readonly IPlanningApplicationRepository planningApplicationRepository;
    private readonly TimeProvider timeProvider;
    private readonly IUserProfileRepository userProfileRepository;
    private readonly IWatchZoneRepository watchZoneRepository;

    public CreateWatchZoneCommandHandler(
        IWatchZoneRepository watchZoneRepository,
        IUserProfileRepository userProfileRepository,
        IPlanItClient planItClient,
        IPlanningApplicationRepository planningApplicationRepository,
        IAuthorityResolver authorityResolver,
        TimeProvider timeProvider)
    {
        this.watchZoneRepository = watchZoneRepository;
        this.userProfileRepository = userProfileRepository;
        this.planItClient = planItClient;
        this.planningApplicationRepository = planningApplicationRepository;
        this.authorityResolver = authorityResolver;
        this.timeProvider = timeProvider;
    }

    public async Task<CreateWatchZoneResult> HandleAsync(CreateWatchZoneCommand command, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(command);

        var profile = await this.userProfileRepository.GetByUserIdAsync(command.UserId, ct).ConfigureAwait(false)
            ?? throw new InvalidOperationException($"User profile not found: {command.UserId}");

        var authorityId = command.AuthorityId
            ?? await this.authorityResolver.ResolveFromCoordinatesAsync(command.Latitude, command.Longitude, ct).ConfigureAwait(false);

        var zone = new WatchZone(
            command.ZoneId,
            command.UserId,
            command.Name,
            new Coordinates(command.Latitude, command.Longitude),
            command.RadiusMetres,
            authorityId);

        await this.watchZoneRepository.SaveAsync(zone, ct).ConfigureAwait(false);

        if (profile.Tier != SubscriptionTier.Free)
        {
            var backfillSince = this.timeProvider.GetUtcNow() - BackfillWindow;

            await foreach (var application in this.planItClient.FetchApplicationsAsync(
                authorityId, backfillSince, ct).ConfigureAwait(false))
            {
                await this.planningApplicationRepository.UpsertAsync(application, ct).ConfigureAwait(false);
            }
        }

        var authorityCode = authorityId.ToString(System.Globalization.CultureInfo.InvariantCulture);
        var nearbyApplications = await this.planningApplicationRepository.FindNearbyAsync(
            authorityCode, command.Latitude, command.Longitude, command.RadiusMetres, ct).ConfigureAwait(false);

        return new CreateWatchZoneResult(nearbyApplications);
    }
}
