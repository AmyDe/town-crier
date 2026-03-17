using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;

namespace TownCrier.Application.Tests.Groups;

public sealed class DeleteGroupCommandHandlerTests
{
    [Test]
    public async Task Should_DeleteGroup_When_OwnerRequests()
    {
        // Arrange
        var group = new GroupBuilder()
            .WithId("group-1")
            .WithOwnerId("owner-1")
            .Build();
        var repository = new FakeGroupRepository();
        await repository.SaveAsync(group, CancellationToken.None);
        var handler = new DeleteGroupCommandHandler(repository);

        // Act
        await handler.HandleAsync(
            new DeleteGroupCommand("owner-1", "group-1"), CancellationToken.None);

        // Assert
        var deleted = await repository.GetByIdAsync("group-1", CancellationToken.None);
        await Assert.That(deleted).IsNull();
    }

    [Test]
    public async Task Should_ThrowUnauthorized_When_NonOwnerDeletes()
    {
        // Arrange
        var group = new GroupBuilder()
            .WithId("group-1")
            .WithOwnerId("owner-1")
            .Build();
        var repository = new FakeGroupRepository();
        await repository.SaveAsync(group, CancellationToken.None);
        var handler = new DeleteGroupCommandHandler(repository);

        // Act & Assert
        await Assert.ThrowsAsync<UnauthorizedGroupOperationException>(
            () => handler.HandleAsync(
                new DeleteGroupCommand("not-owner", "group-1"), CancellationToken.None));
    }
}
