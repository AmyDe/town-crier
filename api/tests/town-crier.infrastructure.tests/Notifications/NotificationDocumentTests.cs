using TownCrier.Domain.Notifications;
using TownCrier.Infrastructure.Notifications;

namespace TownCrier.Infrastructure.Tests.Notifications;

public sealed class NotificationDocumentTests
{
    [Test]
    public async Task Should_RoundTripAllProperties_When_MappingFromDomainAndBack()
    {
        // Arrange
        var notification = new NotificationBuilder()
            .WithUserId("user-42")
            .WithApplicationName("APP/2026/0099")
            .WithWatchZoneId("zone-7")
            .WithApplicationAddress("99 Oak Lane")
            .WithApplicationDescription("Loft conversion")
            .WithApplicationType("Full Planning")
            .WithAuthorityId(55)
            .WithCreatedAt(new DateTimeOffset(2026, 3, 20, 14, 30, 0, TimeSpan.Zero))
            .Build();

        // Act
        var document = NotificationDocument.FromDomain(notification);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.Id).IsEqualTo(notification.Id);
        await Assert.That(roundTripped.UserId).IsEqualTo("user-42");
        await Assert.That(roundTripped.ApplicationName).IsEqualTo("APP/2026/0099");
        await Assert.That(roundTripped.WatchZoneId).IsEqualTo("zone-7");
        await Assert.That(roundTripped.ApplicationAddress).IsEqualTo("99 Oak Lane");
        await Assert.That(roundTripped.ApplicationDescription).IsEqualTo("Loft conversion");
        await Assert.That(roundTripped.ApplicationType).IsEqualTo("Full Planning");
        await Assert.That(roundTripped.AuthorityId).IsEqualTo(55);
        await Assert.That(roundTripped.PushSent).IsEqualTo(notification.PushSent);
        await Assert.That(roundTripped.EmailSent).IsEqualTo(notification.EmailSent);
        await Assert.That(roundTripped.CreatedAt).IsEqualTo(notification.CreatedAt);
    }

    [Test]
    public async Task Should_PreservePushSentFlag_When_NotificationHasPushSent()
    {
        // Arrange
        var notification = new NotificationBuilder().Build();
        notification.MarkPushSent();

        // Act
        var document = NotificationDocument.FromDomain(notification);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.PushSent).IsTrue();
    }

    [Test]
    public async Task Should_PreserveEmailSentFlag_When_NotificationHasEmailSent()
    {
        // Arrange
        var notification = new NotificationBuilder().Build();
        notification.MarkEmailSent();

        // Act
        var document = NotificationDocument.FromDomain(notification);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.EmailSent).IsTrue();
    }

    [Test]
    public async Task Should_DefaultEmailSentToFalse_When_NewlyCreated()
    {
        // Arrange
        var notification = new NotificationBuilder().Build();

        // Act
        var document = NotificationDocument.FromDomain(notification);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.EmailSent).IsFalse();
    }

    [Test]
    public async Task Should_SetUserIdAsPartitionKey_When_MappingFromDomain()
    {
        // Arrange
        var notification = new NotificationBuilder()
            .WithUserId("user-partition-test")
            .Build();

        // Act
        var document = NotificationDocument.FromDomain(notification);

        // Assert
        await Assert.That(document.UserId).IsEqualTo("user-partition-test");
    }

    [Test]
    public async Task Should_SetTtlTo90Days_When_MappingFromDomain()
    {
        // Arrange
        var notification = new NotificationBuilder().Build();

        // Act
        var document = NotificationDocument.FromDomain(notification);

        // Assert
        var ninetyDaysInSeconds = (int)TimeSpan.FromDays(90).TotalSeconds;
        await Assert.That(document.Ttl).IsEqualTo(ninetyDaysInSeconds);
    }
}
