using TownCrier.Domain.Groups;
using TownCrier.Infrastructure.Groups;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.Groups;

public sealed class CosmosGroupInvitationRepositoryTests
{
    [Test]
    public async Task Should_PersistInvitation_When_SaveCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosGroupInvitationRepository(client);
        var invitation = GroupInvitation.Create(
            "inv-1", "group-1", "user@example.com", "user-1",
            DateTimeOffset.UtcNow, TimeSpan.FromDays(7));

        // Act
        await repo.SaveAsync(invitation, CancellationToken.None);

        // Assert
        var result = await repo.GetByIdAsync("inv-1", CancellationToken.None);
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.GroupId).IsEqualTo("group-1");
    }

    [Test]
    public async Task Should_ReturnNull_When_GetByIdForMissingInvitation()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosGroupInvitationRepository(client);

        // Act
        var result = await repo.GetByIdAsync("nonexistent", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_ReturnInvitations_When_GetPendingByGroupIdCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosGroupInvitationRepository(client);
        var invitation = GroupInvitation.Create(
            "inv-1", "group-1", "user@example.com", "user-1",
            DateTimeOffset.UtcNow, TimeSpan.FromDays(7));
        await repo.SaveAsync(invitation, CancellationToken.None);

        // Act -- fake returns all docs in collection
        var result = await repo.GetPendingByGroupIdAsync("group-1", CancellationToken.None);

        // Assert
        await Assert.That(result.Count).IsGreaterThanOrEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnInvitations_When_GetPendingByEmailCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosGroupInvitationRepository(client);
        var invitation = GroupInvitation.Create(
            "inv-1", "group-1", "user@example.com", "user-1",
            DateTimeOffset.UtcNow, TimeSpan.FromDays(7));
        await repo.SaveAsync(invitation, CancellationToken.None);

        // Act
        var result = await repo.GetPendingByEmailAsync("user@example.com", CancellationToken.None);

        // Assert
        await Assert.That(result.Count).IsGreaterThanOrEqualTo(1);
    }
}
