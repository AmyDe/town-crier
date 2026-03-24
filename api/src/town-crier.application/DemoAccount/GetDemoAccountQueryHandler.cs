using TownCrier.Application.PlanningApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.UserProfiles;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.DemoAccount;

public sealed class GetDemoAccountQueryHandler
{
    public const string DemoUserId = "demo|apple-reviewer";
    private const string DemoZoneId = "demo-zone";
    private const string DemoAuthorityName = "Westminster City Council";
    private const int DemoAuthorityId = 441;
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

            var zone = new WatchZone(DemoZoneId, DemoUserId, "Westminster Demo Zone", new Coordinates(DemoLatitude, DemoLongitude), DemoRadiusMetres, DemoAuthorityId);
            await this.watchZoneRepository.SaveAsync(zone, ct).ConfigureAwait(false);

            await this.SeedDemoApplicationsAsync(ct).ConfigureAwait(false);
        }

        var authorityCode = DemoAuthorityId.ToString(System.Globalization.CultureInfo.InvariantCulture);
        var applications = await this.planningApplicationRepository.FindNearbyAsync(
            authorityCode, DemoLatitude, DemoLongitude, DemoRadiusMetres, ct).ConfigureAwait(false);

        var watchZoneResult = new DemoWatchZoneResult(
            DemoZoneId, DemoAuthorityName, DemoLatitude, DemoLongitude, DemoRadiusMetres);

        var applicationResults = applications
            .Select(a => new DemoApplicationResult(a.Uid, a.Name, a.Address, a.Description, a.AppType, a.AppState))
            .ToList();

        return new GetDemoAccountResult(DemoUserId, profile.Tier, watchZoneResult, applicationResults);
    }

    private async Task SeedDemoApplicationsAsync(CancellationToken ct)
    {
        var now = DateTimeOffset.UtcNow;
        var demoApplications = new[]
        {
            new PlanningApplication(
                name: "24/05678/FULL",
                uid: "demo-app-001",
                areaName: DemoAuthorityName,
                areaId: DemoAuthorityId,
                address: "10 Downing Street, London SW1A 2AA",
                postcode: "SW1A 2AA",
                description: "Replacement of entrance door and associated security improvements",
                appType: "Full",
                appState: "Under consideration",
                appSize: null,
                startDate: new DateOnly(2026, 1, 15),
                decidedDate: null,
                consultedDate: new DateOnly(2026, 2, 1),
                longitude: -0.1276,
                latitude: 51.5034,
                url: null,
                link: null,
                lastDifferent: now),
            new PlanningApplication(
                name: "24/05679/FULL",
                uid: "demo-app-002",
                areaName: DemoAuthorityName,
                areaId: DemoAuthorityId,
                address: "Westminster Abbey, 20 Deans Yd, London SW1P 3PA",
                postcode: "SW1P 3PA",
                description: "Installation of new lighting system in the nave and restoration of stonework",
                appType: "Listed Building",
                appState: "Approved",
                appSize: null,
                startDate: new DateOnly(2025, 11, 1),
                decidedDate: new DateOnly(2026, 2, 20),
                consultedDate: new DateOnly(2025, 12, 1),
                longitude: -0.1273,
                latitude: 51.4993,
                url: null,
                link: null,
                lastDifferent: now),
            new PlanningApplication(
                name: "24/05680/FULL",
                uid: "demo-app-003",
                areaName: DemoAuthorityName,
                areaId: DemoAuthorityId,
                address: "Buckingham Palace, London SW1A 1AA",
                postcode: "SW1A 1AA",
                description: "Erection of temporary scaffolding for facade cleaning and minor repairs to roof drainage",
                appType: "Full",
                appState: "Under consideration",
                appSize: null,
                startDate: new DateOnly(2026, 2, 10),
                decidedDate: null,
                consultedDate: null,
                longitude: -0.1419,
                latitude: 51.5014,
                url: null,
                link: null,
                lastDifferent: now),
            new PlanningApplication(
                name: "24/05681/FULL",
                uid: "demo-app-004",
                areaName: DemoAuthorityName,
                areaId: DemoAuthorityId,
                address: "St James's Park, London SW1A 2BJ",
                postcode: "SW1A 2BJ",
                description: "Construction of new accessible footpath and installation of park benches",
                appType: "Full",
                appState: "Refused",
                appSize: null,
                startDate: new DateOnly(2025, 10, 5),
                decidedDate: new DateOnly(2026, 1, 30),
                consultedDate: new DateOnly(2025, 11, 1),
                longitude: -0.1340,
                latitude: 51.5025,
                url: null,
                link: null,
                lastDifferent: now),
        };

        foreach (var application in demoApplications)
        {
            await this.planningApplicationRepository.UpsertAsync(application, ct).ConfigureAwait(false);
        }
    }
}
