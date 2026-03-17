using TownCrier.Application.Groups;

namespace TownCrier.Application.Tests.Groups;

public sealed class GetGroupQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnGroup_When_UserIsMember()
    {
        // Arrange
        var group = new GroupBuilder()
            .WithId("group-1")
            .WithName("Elm Street Neighbours")
            .WithOwnerId("user-1")
            .Build();
        var repository = new FakeGroupRepository();
        await repository.SaveAsync(group, CancellationToken.None);
        var handler = new GetGroupQueryHandler(repository);

        // Act
        var result = await handler.HandleAsync(
            new GetGroupQuery("user-1", "group-1"), CancellationToken.None);

        // Assert
        await Assert.That(result.GroupId).IsEqualTo("group-1");
        await Assert.That(result.Name).IsEqualTo("Elm Street Neighbours");
        await Assert.That(result.OwnerId).IsEqualTo("user-1");
        await Assert.That(result.Members).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_ThrowGroupNotFound_When_UserIsNotMember()
    {
        // Arrange
        var group = new GroupBuilder()
            .WithId("group-1")
            .WithOwnerId("user-1")
            .Build();
        var repository = new FakeGroupRepository();
        await repository.SaveAsync(group, CancellationToken.None);
        var handler = new GetGroupQueryHandler(repository);

        // Act & Assert
        await Assert.ThrowsAsync<GroupNotFoundException>(
            () => handler.HandleAsync(
                new GetGroupQuery("stranger", "group-1"), CancellationToken.None));
    }

    [Test]
    public async Task Should_ThrowGroupNotFound_When_GroupDoesNotExist()
    {
        // Arrange
        var repository = new FakeGroupRepository();
        var handler = new GetGroupQueryHandler(repository);

        // Act & Assert
        await Assert.ThrowsAsync<GroupNotFoundException>(
            () => handler.HandleAsync(
                new GetGroupQuery("user-1", "nonexistent"), CancellationToken.None));
    }
}
