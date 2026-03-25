using TownCrier.Application.Tests.Polling;
using TownCrier.Application.WatchZones;

namespace TownCrier.Application.Tests.WatchZones;

public sealed class DeleteWatchZoneCommandHandlerTests
{
    private readonly FakeWatchZoneRepository watchZoneRepository = new();

    [Test]
    public async Task Should_DeleteZone_When_ZoneExistsForUser()
    {
        // Arrange
        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("My Zone")
            .Build();
        this.watchZoneRepository.Add(zone);

        var handler = new DeleteWatchZoneCommandHandler(this.watchZoneRepository);
        var command = new DeleteWatchZoneCommand("user-1", "zone-1");

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var remaining = await this.watchZoneRepository.GetByUserIdAsync("user-1", CancellationToken.None);
        await Assert.That(remaining).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ThrowWatchZoneNotFound_When_ZoneDoesNotExist()
    {
        // Arrange
        var handler = new DeleteWatchZoneCommandHandler(this.watchZoneRepository);
        var command = new DeleteWatchZoneCommand("user-1", "nonexistent-zone");

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

        var handler = new DeleteWatchZoneCommandHandler(this.watchZoneRepository);
        var command = new DeleteWatchZoneCommand("user-1", "zone-1");

        // Act & Assert
        await Assert.ThrowsAsync<WatchZoneNotFoundException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_NotDeleteOtherZones_When_DeletingOneZone()
    {
        // Arrange
        var zone1 = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("Zone One")
            .Build();
        var zone2 = new WatchZoneBuilder()
            .WithId("zone-2")
            .WithUserId("user-1")
            .WithName("Zone Two")
            .Build();
        this.watchZoneRepository.Add(zone1);
        this.watchZoneRepository.Add(zone2);

        var handler = new DeleteWatchZoneCommandHandler(this.watchZoneRepository);
        var command = new DeleteWatchZoneCommand("user-1", "zone-1");

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var remaining = await this.watchZoneRepository.GetByUserIdAsync("user-1", CancellationToken.None);
        await Assert.That(remaining).HasCount().EqualTo(1);
        await Assert.That(remaining.First().Id).IsEqualTo("zone-2");
    }
}
