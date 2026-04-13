using TownCrier.Application.Tests.Polling;
using TownCrier.Application.WatchZones;

namespace TownCrier.Application.Tests.WatchZones;

public sealed class GetApplicationsByZoneQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnNearbyApplications_When_ZoneExists()
    {
        // Arrange — zone centred on Camden Town, 1 km radius
        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("My Zone")
            .WithCentre(51.5390, -0.1426)
            .WithRadiusMetres(1000)
            .WithAuthorityId(42)
            .Build();

        var watchZoneRepo = new FakeWatchZoneRepository();
        watchZoneRepo.Add(zone);

        // Application ~200m from centre (inside zone)
        var nearby = new PlanningApplicationBuilder()
            .WithName("nearby-app")
            .WithUid("uid-nearby")
            .WithAreaId(42)
            .WithCoordinates(51.5380, -0.1410)
            .Build();

        // Application ~5km from centre (outside zone)
        var far = new PlanningApplicationBuilder()
            .WithName("far-app")
            .WithUid("uid-far")
            .WithAreaId(42)
            .WithCoordinates(51.5074, -0.1278)
            .Build();

        var appRepo = new FakePlanningApplicationRepository();
        await appRepo.UpsertAsync(nearby, CancellationToken.None);
        await appRepo.UpsertAsync(far, CancellationToken.None);

        var handler = new GetApplicationsByZoneQueryHandler(watchZoneRepo, appRepo);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationsByZoneQuery("user-1", "zone-1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Count).IsEqualTo(1);
        await Assert.That(result[0].Uid).IsEqualTo("uid-nearby");
    }
}
