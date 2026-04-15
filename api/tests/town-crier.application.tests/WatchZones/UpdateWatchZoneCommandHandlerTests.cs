using TownCrier.Application.Tests.Polling;
using TownCrier.Application.WatchZones;

namespace TownCrier.Application.Tests.WatchZones;

public sealed class UpdateWatchZoneCommandHandlerTests
{
    private readonly FakeWatchZoneRepository watchZoneRepository = new();

    [Test]
    public async Task Should_UpdateName_When_NameProvided()
    {
        // Arrange
        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("Original Name")
            .Build();
        this.watchZoneRepository.Add(zone);

        var handler = new UpdateWatchZoneCommandHandler(this.watchZoneRepository);
        var command = new UpdateWatchZoneCommand("user-1", "zone-1", Name: "Updated Name");

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Zone.Name).IsEqualTo("Updated Name");
        await Assert.That(result.Zone.Id).IsEqualTo("zone-1");
    }

    [Test]
    public async Task Should_ThrowWatchZoneNotFound_When_ZoneDoesNotExist()
    {
        // Arrange
        var handler = new UpdateWatchZoneCommandHandler(this.watchZoneRepository);
        var command = new UpdateWatchZoneCommand("user-1", "nonexistent-zone", Name: "Updated");

        // Act & Assert
        await Assert.ThrowsAsync<WatchZoneNotFoundException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_ThrowWatchZoneNotFound_When_ZoneBelongsToDifferentUser()
    {
        // Arrange
        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-2")
            .WithName("Other User Zone")
            .Build();
        this.watchZoneRepository.Add(zone);

        var handler = new UpdateWatchZoneCommandHandler(this.watchZoneRepository);
        var command = new UpdateWatchZoneCommand("user-1", "zone-1", Name: "Hijacked");

        // Act & Assert
        await Assert.ThrowsAsync<WatchZoneNotFoundException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_ApplyPartialUpdate_When_OnlyRadiusProvided()
    {
        // Arrange
        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("My Zone")
            .WithCentre(51.5074, -0.1278)
            .WithRadiusMetres(1000)
            .WithAuthorityId(42)
            .Build();
        this.watchZoneRepository.Add(zone);

        var handler = new UpdateWatchZoneCommandHandler(this.watchZoneRepository);
        var command = new UpdateWatchZoneCommand("user-1", "zone-1", RadiusMetres: 2500);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert — only radius changed, everything else preserved
        await Assert.That(result.Zone.RadiusMetres).IsEqualTo(2500);
        await Assert.That(result.Zone.Name).IsEqualTo("My Zone");
        await Assert.That(result.Zone.Latitude).IsEqualTo(51.5074);
        await Assert.That(result.Zone.Longitude).IsEqualTo(-0.1278);
        await Assert.That(result.Zone.AuthorityId).IsEqualTo(42);
    }

    [Test]
    public async Task Should_UpdateAllFields_When_AllFieldsProvided()
    {
        // Arrange
        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("Original")
            .WithCentre(51.5074, -0.1278)
            .WithRadiusMetres(1000)
            .WithAuthorityId(10)
            .Build();
        this.watchZoneRepository.Add(zone);

        var handler = new UpdateWatchZoneCommandHandler(this.watchZoneRepository);
        var command = new UpdateWatchZoneCommand(
            "user-1",
            "zone-1",
            Name: "Renamed",
            Latitude: 52.0,
            Longitude: -1.0,
            RadiusMetres: 3000,
            AuthorityId: 99);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Zone.Name).IsEqualTo("Renamed");
        await Assert.That(result.Zone.Latitude).IsEqualTo(52.0);
        await Assert.That(result.Zone.Longitude).IsEqualTo(-1.0);
        await Assert.That(result.Zone.RadiusMetres).IsEqualTo(3000);
        await Assert.That(result.Zone.AuthorityId).IsEqualTo(99);
    }

    [Test]
    public async Task Should_PersistUpdatedZone_When_UpdateSucceeds()
    {
        // Arrange
        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("Old Name")
            .Build();
        this.watchZoneRepository.Add(zone);

        var handler = new UpdateWatchZoneCommandHandler(this.watchZoneRepository);
        var command = new UpdateWatchZoneCommand("user-1", "zone-1", Name: "New Name");

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert — verify the repository was actually updated
        var saved = await this.watchZoneRepository.GetByUserAndZoneIdAsync("user-1", "zone-1", CancellationToken.None);
        await Assert.That(saved).IsNotNull();
        await Assert.That(saved!.Name).IsEqualTo("New Name");
    }
}
