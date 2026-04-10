using TownCrier.Application.PlanningApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.UserProfiles;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.DemoAccount;

public sealed class GetDemoAccountQueryHandler
{
    public const string DemoUserId = "demo|apple-reviewer";
    private const string DemoZoneId = "demo-zone";
    private const double DemoLatitude = 51.4975;
    private const double DemoLongitude = -0.1357;
    private const double DemoRadiusMetres = 2000;

    private readonly IUserProfileRepository userProfileRepository;
    private readonly IWatchZoneRepository watchZoneRepository;
    private readonly IPlanningApplicationRepository planningApplicationRepository;

    public GetDemoAccountQueryHandler(
        IUserProfileRepository userProfileRepository,
        IWatchZoneRepository watchZoneRepository,
        IPlanningApplicationRepository planningApplicationRepository)
    {
        this.userProfileRepository = userProfileRepository;
        this.watchZoneRepository = watchZoneRepository;
        this.planningApplicationRepository = planningApplicationRepository;
    }

    public async Task<GetDemoAccountResult> HandleAsync(GetDemoAccountQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var profile = await this.userProfileRepository.GetByUserIdAsync(DemoUserId, ct).ConfigureAwait(false);

        if (profile is null)
        {
            profile = UserProfile.Register(DemoUserId);
            profile.ActivateSubscription(SubscriptionTier.Pro, DateTimeOffset.UtcNow.AddYears(10));
            await this.userProfileRepository.SaveAsync(profile, ct).ConfigureAwait(false);

            var zone = new WatchZone(DemoZoneId, DemoUserId, "Westminster Demo Zone", new Coordinates(DemoLatitude, DemoLongitude), DemoRadiusMetres, DemoSeedData.AuthorityId, DateTimeOffset.MinValue);
            await this.watchZoneRepository.SaveAsync(zone, ct).ConfigureAwait(false);

            await this.SeedDemoApplicationsAsync(ct).ConfigureAwait(false);
        }

        var authorityCode = DemoSeedData.AuthorityId.ToString(System.Globalization.CultureInfo.InvariantCulture);
        var applications = await this.planningApplicationRepository.FindNearbyAsync(
            authorityCode, DemoLatitude, DemoLongitude, DemoRadiusMetres, ct).ConfigureAwait(false);

        var watchZoneResult = new DemoWatchZoneResult(
            DemoZoneId, DemoSeedData.AuthorityName, DemoLatitude, DemoLongitude, DemoRadiusMetres);

        var applicationResults = applications
            .Select(a => new DemoApplicationResult(a.Uid, a.Name, a.Address, a.Description, a.AppType, a.AppState))
            .ToList();

        return new GetDemoAccountResult(DemoUserId, profile.Tier, watchZoneResult, applicationResults);
    }

    private async Task SeedDemoApplicationsAsync(CancellationToken ct)
    {
        var demoApplications = DemoSeedData.CreateApplications(DateTimeOffset.UtcNow);

        foreach (var application in demoApplications)
        {
            await this.planningApplicationRepository.UpsertAsync(application, ct).ConfigureAwait(false);
        }
    }
}
