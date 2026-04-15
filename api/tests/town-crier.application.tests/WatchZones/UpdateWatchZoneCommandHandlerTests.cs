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
}
