using TownCrier.Application.Groups;
using TownCrier.Domain.Groups;

namespace TownCrier.Application.Tests.Groups;

public sealed class AcceptInvitationCommandHandlerTests
{
    [Test]
    public async Task Should_AddUserToGroup_When_InvitationAccepted()
    {
        // Arrange
        var now = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);
        var group = new GroupBuilder()
            .WithId("group-1")
            .WithOwnerId("owner-1")
            .WithCreatedAt(now)
            .Build();
        var groupRepo = new FakeGroupRepository();
        await groupRepo.SaveAsync(group, CancellationToken.None);

        var invitation = GroupInvitation.Create(
            "invite-1",
            "group-1",
            "new@example.com",
            "owner-1",
            now,
            TimeSpan.FromDays(7));
        var invitationRepo = new FakeGroupInvitationRepository();
        await invitationRepo.SaveAsync(invitation, CancellationToken.None);

        var acceptTime = now.AddHours(1);
        var timeProvider = new FakeTimeProvider(acceptTime);
        var handler = new AcceptInvitationCommandHandler(groupRepo, invitationRepo, timeProvider);

        // Act
        await handler.HandleAsync(
            new AcceptInvitationCommand("new-user-1", "invite-1"), CancellationToken.None);

        // Assert
        var updatedGroup = await groupRepo.GetByIdAsync("group-1", CancellationToken.None);
        await Assert.That(updatedGroup!.Members).HasCount().EqualTo(2);
        await Assert.That(updatedGroup.IsMember("new-user-1")).IsTrue();

        var updatedInvitation = await invitationRepo.GetByIdAsync("invite-1", CancellationToken.None);
        await Assert.That(updatedInvitation!.Status).IsEqualTo(InvitationStatus.Accepted);
    }

    [Test]
    public async Task Should_ThrowInvalidOperation_When_InvitationExpired()
    {
        // Arrange
        var createdAt = new DateTimeOffset(2026, 3, 1, 10, 0, 0, TimeSpan.Zero);
        var group = new GroupBuilder()
            .WithId("group-1")
            .WithOwnerId("owner-1")
            .Build();
        var groupRepo = new FakeGroupRepository();
        await groupRepo.SaveAsync(group, CancellationToken.None);

        var invitation = GroupInvitation.Create(
            "invite-1",
            "group-1",
            "late@example.com",
            "owner-1",
            createdAt,
            TimeSpan.FromDays(7));
        var invitationRepo = new FakeGroupInvitationRepository();
        await invitationRepo.SaveAsync(invitation, CancellationToken.None);

        // 8 days later — expired
        var acceptTime = createdAt.AddDays(8);
        var timeProvider = new FakeTimeProvider(acceptTime);
        var handler = new AcceptInvitationCommandHandler(groupRepo, invitationRepo, timeProvider);

        // Act & Assert
        await Assert.ThrowsAsync<InvalidOperationException>(
            () => handler.HandleAsync(
                new AcceptInvitationCommand("late-user", "invite-1"), CancellationToken.None));
    }

    [Test]
    public async Task Should_ThrowInvalidOperation_When_UserAlreadyMember()
    {
        // Arrange
        var now = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);
        var group = new GroupBuilder()
            .WithId("group-1")
            .WithOwnerId("owner-1")
            .WithCreatedAt(now)
            .Build();
        var groupRepo = new FakeGroupRepository();
        await groupRepo.SaveAsync(group, CancellationToken.None);

        var invitation = GroupInvitation.Create(
            "invite-1",
            "group-1",
            "owner@example.com",
            "owner-1",
            now,
            TimeSpan.FromDays(7));
        var invitationRepo = new FakeGroupInvitationRepository();
        await invitationRepo.SaveAsync(invitation, CancellationToken.None);

        var timeProvider = new FakeTimeProvider(now.AddMinutes(5));
        var handler = new AcceptInvitationCommandHandler(groupRepo, invitationRepo, timeProvider);

        // Act & Assert — owner-1 is already a member
        await Assert.ThrowsAsync<InvalidOperationException>(
            () => handler.HandleAsync(
                new AcceptInvitationCommand("owner-1", "invite-1"), CancellationToken.None));
    }
}
