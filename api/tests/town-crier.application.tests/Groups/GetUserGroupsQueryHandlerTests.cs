using TownCrier.Application.Groups;

namespace TownCrier.Application.Tests.Groups;

public sealed class GetUserGroupsQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnGroups_When_UserIsMember()
    {
        // Arrange
        var now = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);
        var group1 = new GroupBuilder()
            .WithId("group-1")
            .WithName("Elm Street")
            .WithOwnerId("user-1")
            .WithCreatedAt(now)
            .Build();
        var group2 = new GroupBuilder()
            .WithId("group-2")
            .WithName("Parish Council")
            .WithOwnerId("other-owner")
            .WithCreatedAt(now)
            .Build();
        group2.AddMember("user-1", now);

        var repository = new FakeGroupRepository();
        await repository.SaveAsync(group1, CancellationToken.None);
        await repository.SaveAsync(group2, CancellationToken.None);
        var handler = new GetUserGroupsQueryHandler(repository);

        // Act
        var result = await handler.HandleAsync(
            new GetUserGroupsQuery("user-1"), CancellationToken.None);

        // Assert
        await Assert.That(result.Groups).HasCount().EqualTo(2);
        await Assert.That(result.Groups[0].Name).IsEqualTo("Elm Street");
        await Assert.That(result.Groups[0].Role).IsEqualTo("Owner");
        await Assert.That(result.Groups[1].Name).IsEqualTo("Parish Council");
        await Assert.That(result.Groups[1].Role).IsEqualTo("Member");
        await Assert.That(result.Groups[1].MemberCount).IsEqualTo(2);
    }

    [Test]
    public async Task Should_ReturnEmpty_When_UserHasNoGroups()
    {
        // Arrange
        var repository = new FakeGroupRepository();
        var handler = new GetUserGroupsQueryHandler(repository);

        // Act
        var result = await handler.HandleAsync(
            new GetUserGroupsQuery("lonely-user"), CancellationToken.None);

        // Assert
        await Assert.That(result.Groups).HasCount().EqualTo(0);
    }
}
