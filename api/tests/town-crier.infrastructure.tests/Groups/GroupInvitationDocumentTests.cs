using TownCrier.Domain.Groups;
using TownCrier.Infrastructure.Groups;

namespace TownCrier.Infrastructure.Tests.Groups;

public sealed class GroupInvitationDocumentTests
{
    [Test]
    public async Task Should_RoundTripAllProperties_When_MappingFromDomainAndBack()
    {
        // Arrange
        var invitation = GroupInvitation.Create(
            "inv-1",
            "group-1",
            "jane@example.com",
            "owner-1",
            new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero),
            TimeSpan.FromDays(7));

        // Act
        var document = GroupInvitationDocument.FromDomain(invitation);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.Id).IsEqualTo(invitation.Id);
        await Assert.That(roundTripped.GroupId).IsEqualTo(invitation.GroupId);
        await Assert.That(roundTripped.InviteeEmail).IsEqualTo(invitation.InviteeEmail);
        await Assert.That(roundTripped.InvitedByUserId).IsEqualTo(invitation.InvitedByUserId);
        await Assert.That(roundTripped.Status).IsEqualTo(InvitationStatus.Pending);
        await Assert.That(roundTripped.CreatedAt).IsEqualTo(invitation.CreatedAt);
        await Assert.That(roundTripped.ExpiresAt).IsEqualTo(invitation.ExpiresAt);
    }

    [Test]
    public async Task Should_SetTypeDiscriminator_When_MappingFromDomain()
    {
        // Arrange
        var invitation = GroupInvitation.Create(
            "inv-1",
            "group-1",
            "jane@example.com",
            "owner-1",
            new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero),
            TimeSpan.FromDays(7));

        // Act
        var document = GroupInvitationDocument.FromDomain(invitation);

        // Assert
        await Assert.That(document.Type).IsEqualTo("invitation");
    }

    [Test]
    public async Task Should_SetOwnerIdFromInvitedByUserId_When_MappingFromDomain()
    {
        // Arrange
        var invitation = GroupInvitation.Create(
            "inv-1",
            "group-1",
            "jane@example.com",
            "owner-partition-test",
            new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero),
            TimeSpan.FromDays(7));

        // Act
        var document = GroupInvitationDocument.FromDomain(invitation);

        // Assert
        await Assert.That(document.OwnerId).IsEqualTo("owner-partition-test");
    }

    [Test]
    public async Task Should_PreserveAcceptedStatus_When_MappingAfterAcceptance()
    {
        // Arrange
        var now = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);
        var invitation = GroupInvitation.Create(
            "inv-1",
            "group-1",
            "jane@example.com",
            "owner-1",
            now,
            TimeSpan.FromDays(7));
        invitation.Accept(now.AddHours(1));

        // Act
        var document = GroupInvitationDocument.FromDomain(invitation);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.Status).IsEqualTo(InvitationStatus.Accepted);
    }
}
