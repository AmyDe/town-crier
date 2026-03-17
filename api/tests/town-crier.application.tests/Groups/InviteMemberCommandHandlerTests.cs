using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;

namespace TownCrier.Application.Tests.Groups;

public sealed class InviteMemberCommandHandlerTests
{
    [Test]
    public async Task Should_CreateInvitation_When_OwnerInvites()
    {
        // Arrange
        var group = new GroupBuilder()
            .WithId("group-1")
            .WithOwnerId("owner-1")
            .Build();
        var groupRepo = new FakeGroupRepository();
        await groupRepo.SaveAsync(group, CancellationToken.None);
        var invitationRepo = new FakeGroupInvitationRepository();
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
        var handler = new InviteMemberCommandHandler(groupRepo, invitationRepo, timeProvider);

        var command = new InviteMemberCommand(
            RequestingUserId: "owner-1",
            GroupId: "group-1",
            InvitationId: "invite-1",
            InviteeEmail: "neighbour@example.com");

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.InvitationId).IsEqualTo("invite-1");
        await Assert.That(result.InviteeEmail).IsEqualTo("neighbour@example.com");
        await Assert.That(invitationRepo.Count).IsEqualTo(1);
    }

    [Test]
    public async Task Should_ThrowUnauthorized_When_NonOwnerInvites()
    {
        // Arrange
        var group = new GroupBuilder()
            .WithId("group-1")
            .WithOwnerId("owner-1")
            .Build();
        var groupRepo = new FakeGroupRepository();
        await groupRepo.SaveAsync(group, CancellationToken.None);
        var invitationRepo = new FakeGroupInvitationRepository();
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
        var handler = new InviteMemberCommandHandler(groupRepo, invitationRepo, timeProvider);

        var command = new InviteMemberCommand(
            RequestingUserId: "not-owner",
            GroupId: "group-1",
            InvitationId: "invite-1",
            InviteeEmail: "someone@example.com");

        // Act & Assert
        await Assert.ThrowsAsync<UnauthorizedGroupOperationException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }
}
