using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;

namespace TownCrier.Application.Tests.Groups;

public sealed class RemoveGroupMemberCommandHandlerTests
{
    [Test]
    public async Task Should_RemoveMember_When_OwnerRequests()
    {
        // Arrange
        var now = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);
        var group = new GroupBuilder()
            .WithId("group-1")
            .WithOwnerId("owner-1")
            .WithCreatedAt(now)
            .Build();
        group.AddMember("member-1", now);
        var repository = new FakeGroupRepository();
        await repository.SaveAsync(group, CancellationToken.None);
        var handler = new RemoveGroupMemberCommandHandler(repository);

        // Act
        await handler.HandleAsync(
            new RemoveGroupMemberCommand("owner-1", "group-1", "member-1"),
            CancellationToken.None);

        // Assert
        var updated = await repository.GetByIdAsync("group-1", CancellationToken.None);
        await Assert.That(updated!.Members).HasCount().EqualTo(1);
        await Assert.That(updated.IsMember("member-1")).IsFalse();
    }

    [Test]
    public async Task Should_ThrowUnauthorized_When_NonOwnerRemoves()
    {
        // Arrange
        var now = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);
        var group = new GroupBuilder()
            .WithId("group-1")
            .WithOwnerId("owner-1")
            .WithCreatedAt(now)
            .Build();
        group.AddMember("member-1", now);
        var repository = new FakeGroupRepository();
        await repository.SaveAsync(group, CancellationToken.None);
        var handler = new RemoveGroupMemberCommandHandler(repository);

        // Act & Assert
        await Assert.ThrowsAsync<UnauthorizedGroupOperationException>(
            () => handler.HandleAsync(
                new RemoveGroupMemberCommand("member-1", "group-1", "owner-1"),
                CancellationToken.None));
    }

    [Test]
    public async Task Should_ThrowInvalidOperation_When_RemovingOwner()
    {
        // Arrange
        var group = new GroupBuilder()
            .WithId("group-1")
            .WithOwnerId("owner-1")
            .Build();
        var repository = new FakeGroupRepository();
        await repository.SaveAsync(group, CancellationToken.None);
        var handler = new RemoveGroupMemberCommandHandler(repository);

        // Act & Assert
        await Assert.ThrowsAsync<InvalidOperationException>(
            () => handler.HandleAsync(
                new RemoveGroupMemberCommand("owner-1", "group-1", "owner-1"),
                CancellationToken.None));
    }
}
