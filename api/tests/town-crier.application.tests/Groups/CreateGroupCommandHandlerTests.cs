using TownCrier.Application.Groups;

namespace TownCrier.Application.Tests.Groups;

public sealed class CreateGroupCommandHandlerTests
{
    [Test]
    public async Task Should_CreateGroup_When_ValidCommand()
    {
        // Arrange
        var repository = new FakeGroupRepository();
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
        var handler = new CreateGroupCommandHandler(repository, timeProvider);

        var command = new CreateGroupCommand(
            UserId: "user-1",
            GroupId: "group-1",
            Name: "Elm Street Neighbours",
            Latitude: 51.5074,
            Longitude: -0.1278,
            RadiusMetres: 2000,
            AuthorityId: 42);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.GroupId).IsEqualTo("group-1");
        await Assert.That(result.Name).IsEqualTo("Elm Street Neighbours");
        await Assert.That(repository.Count).IsEqualTo(1);

        var saved = await repository.GetByIdAsync("group-1", CancellationToken.None);
        await Assert.That(saved).IsNotNull();
        await Assert.That(saved!.OwnerId).IsEqualTo("user-1");
        await Assert.That(saved.Members).HasCount().EqualTo(1);
        await Assert.That(saved.Members[0].Role).IsEqualTo(Domain.Groups.GroupRole.Owner);
    }

    [Test]
    public async Task Should_SetOwnerAsMember_When_GroupCreated()
    {
        // Arrange
        var repository = new FakeGroupRepository();
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
        var handler = new CreateGroupCommandHandler(repository, timeProvider);

        var command = new CreateGroupCommand(
            UserId: "owner-abc",
            GroupId: "grp-99",
            Name: "Parish Council Watch",
            Latitude: 52.0,
            Longitude: -1.0,
            RadiusMetres: 3000,
            AuthorityId: 7);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = await repository.GetByIdAsync("grp-99", CancellationToken.None);
        await Assert.That(saved!.IsMember("owner-abc")).IsTrue();
        await Assert.That(saved.Members[0].UserId).IsEqualTo("owner-abc");
    }
}
