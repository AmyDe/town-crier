using TownCrier.Application.Tests.Polling;
using TownCrier.Application.WatchZones;

namespace TownCrier.Application.Tests.WatchZones;

public sealed class ListWatchZonesQueryHandlerTests
{
    private readonly FakeWatchZoneRepository watchZoneRepository = new();

    [Test]
    public async Task Should_ReturnUserZones_When_UserHasZones()
    {
        // Arrange
        var zone1 = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("Home")
            .WithCentre(51.5074, -0.1278)
            .WithRadiusMetres(5000)
            .WithAuthorityId(42)
            .Build();
        var zone2 = new WatchZoneBuilder()
            .WithId("zone-2")
            .WithUserId("user-1")
            .WithName("Office")
            .WithCentre(51.5155, -0.1419)
            .WithRadiusMetres(3000)
            .WithAuthorityId(42)
            .Build();
        this.watchZoneRepository.Add(zone1);
        this.watchZoneRepository.Add(zone2);

        var handler = new ListWatchZonesQueryHandler(this.watchZoneRepository);

        // Act
        var result = await handler.HandleAsync(new ListWatchZonesQuery("user-1"), CancellationToken.None);

        // Assert
        await Assert.That(result.Zones).HasCount().EqualTo(2);
        await Assert.That(result.Zones.First(z => z.Id == "zone-1").Name).IsEqualTo("Home");
        await Assert.That(result.Zones.First(z => z.Id == "zone-2").Name).IsEqualTo("Office");
    }

    [Test]
    public async Task Should_ReturnEmptyList_When_UserHasNoZones()
    {
        // Arrange
        var handler = new ListWatchZonesQueryHandler(this.watchZoneRepository);

        // Act
        var result = await handler.HandleAsync(new ListWatchZonesQuery("user-1"), CancellationToken.None);

        // Assert
        await Assert.That(result.Zones).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ExcludeOtherUsersZones_When_ListingZones()
    {
        // Arrange
        var myZone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("My Zone")
            .Build();
        var otherZone = new WatchZoneBuilder()
            .WithId("zone-2")
            .WithUserId("user-2")
            .WithName("Other Zone")
            .Build();
        this.watchZoneRepository.Add(myZone);
        this.watchZoneRepository.Add(otherZone);

        var handler = new ListWatchZonesQueryHandler(this.watchZoneRepository);

        // Act
        var result = await handler.HandleAsync(new ListWatchZonesQuery("user-1"), CancellationToken.None);

        // Assert
        await Assert.That(result.Zones).HasCount().EqualTo(1);
        await Assert.That(result.Zones.First().Name).IsEqualTo("My Zone");
    }
}
