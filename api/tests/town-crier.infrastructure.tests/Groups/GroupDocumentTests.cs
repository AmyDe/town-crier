using TownCrier.Domain.Geocoding;
using TownCrier.Domain.Groups;
using TownCrier.Infrastructure.Groups;

namespace TownCrier.Infrastructure.Tests.Groups;

public sealed class GroupDocumentTests
{
    [Test]
    public async Task Should_RoundTripAllProperties_When_MappingFromDomainAndBack()
    {
        // Arrange
        var group = Group.Create(
            "group-1",
            "Test Group",
            "owner-1",
            new Coordinates(51.5074, -0.1278),
            5000,
            42,
            new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));

        // Act
        var document = GroupDocument.FromDomain(group);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.Id).IsEqualTo(group.Id);
        await Assert.That(roundTripped.Name).IsEqualTo(group.Name);
        await Assert.That(roundTripped.OwnerId).IsEqualTo(group.OwnerId);
        await Assert.That(roundTripped.Centre.Latitude).IsEqualTo(group.Centre.Latitude);
        await Assert.That(roundTripped.Centre.Longitude).IsEqualTo(group.Centre.Longitude);
        await Assert.That(roundTripped.RadiusMetres).IsEqualTo(group.RadiusMetres);
        await Assert.That(roundTripped.AuthorityId).IsEqualTo(group.AuthorityId);
        await Assert.That(roundTripped.CreatedAt).IsEqualTo(group.CreatedAt);
    }

    [Test]
    public async Task Should_SetTypeDiscriminator_When_MappingFromDomain()
    {
        // Arrange
        var group = Group.Create(
            "group-1",
            "Test Group",
            "owner-1",
            new Coordinates(51.5074, -0.1278),
            5000,
            42,
            new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));

        // Act
        var document = GroupDocument.FromDomain(group);

        // Assert
        await Assert.That(document.Type).IsEqualTo("group");
    }

    [Test]
    public async Task Should_SetOwnerIdAsPartitionKey_When_MappingFromDomain()
    {
        // Arrange
        var group = Group.Create(
            "group-1",
            "Test Group",
            "owner-partition-test",
            new Coordinates(51.5074, -0.1278),
            5000,
            42,
            new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));

        // Act
        var document = GroupDocument.FromDomain(group);

        // Assert
        await Assert.That(document.OwnerId).IsEqualTo("owner-partition-test");
    }

    [Test]
    public async Task Should_PreserveMembers_When_MappingFromDomainAndBack()
    {
        // Arrange
        var group = Group.Create(
            "group-1",
            "Test Group",
            "owner-1",
            new Coordinates(51.5074, -0.1278),
            5000,
            42,
            new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
        group.AddMember("member-1", new DateTimeOffset(2026, 3, 18, 10, 0, 0, TimeSpan.Zero));

        // Act
        var document = GroupDocument.FromDomain(group);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.Members).HasCount().EqualTo(2);
        await Assert.That(roundTripped.Members[0].UserId).IsEqualTo("owner-1");
        await Assert.That(roundTripped.Members[0].Role).IsEqualTo(GroupRole.Owner);
        await Assert.That(roundTripped.Members[1].UserId).IsEqualTo("member-1");
        await Assert.That(roundTripped.Members[1].Role).IsEqualTo(GroupRole.Member);
    }
}
