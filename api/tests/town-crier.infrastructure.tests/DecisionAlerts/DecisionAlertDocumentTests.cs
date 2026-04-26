using TownCrier.Domain.DecisionAlerts;
using TownCrier.Infrastructure.DecisionAlerts;

namespace TownCrier.Infrastructure.Tests.DecisionAlerts;

public sealed class DecisionAlertDocumentTests
{
    private static readonly DateTimeOffset March2026 = new(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_CreateCompositeId_When_MappedFromDomain()
    {
        // Arrange
        var alert = DecisionAlert.Create(
            "user-1", "app-uid-001", "Test App", "123 High St", "Permitted", March2026);

        // Act
        var document = DecisionAlertDocument.FromDomain(alert);

        // Assert
        await Assert.That(document.Id).IsEqualTo("user-1:app-uid-001");
    }

    [Test]
    public async Task Should_SetUserIdAsPartitionKey_When_MappedFromDomain()
    {
        // Arrange
        var alert = DecisionAlert.Create(
            "user-partition-test", "app-uid-001", "Test App", "123 High St", "Permitted", March2026);

        // Act
        var document = DecisionAlertDocument.FromDomain(alert);

        // Assert
        await Assert.That(document.UserId).IsEqualTo("user-partition-test");
    }

    [Test]
    public async Task Should_PreserveAllFields_When_MappedFromDomain()
    {
        // Arrange
        var alert = DecisionAlert.Create(
            "user-1", "app-uid-001", "Test App", "123 High St", "Rejected", March2026);

        // Act
        var document = DecisionAlertDocument.FromDomain(alert);

        // Assert
        await Assert.That(document.EntityId).IsEqualTo(alert.Id);
        await Assert.That(document.UserId).IsEqualTo("user-1");
        await Assert.That(document.ApplicationUid).IsEqualTo("app-uid-001");
        await Assert.That(document.ApplicationName).IsEqualTo("Test App");
        await Assert.That(document.ApplicationAddress).IsEqualTo("123 High St");
        await Assert.That(document.Decision).IsEqualTo("Rejected");
        await Assert.That(document.PushSent).IsFalse();
        await Assert.That(document.CreatedAt).IsEqualTo(March2026);
    }

    [Test]
    public async Task Should_RoundTripAllProperties_When_MappingFromDomainAndBack()
    {
        // Arrange
        var alert = DecisionAlert.Create(
            "user-1", "app-uid-001", "Test App", "123 High St", "Permitted", March2026);

        // Act
        var document = DecisionAlertDocument.FromDomain(alert);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.Id).IsEqualTo(alert.Id);
        await Assert.That(roundTripped.UserId).IsEqualTo("user-1");
        await Assert.That(roundTripped.ApplicationUid).IsEqualTo("app-uid-001");
        await Assert.That(roundTripped.ApplicationName).IsEqualTo("Test App");
        await Assert.That(roundTripped.ApplicationAddress).IsEqualTo("123 High St");
        await Assert.That(roundTripped.Decision).IsEqualTo("Permitted");
        await Assert.That(roundTripped.PushSent).IsFalse();
        await Assert.That(roundTripped.CreatedAt).IsEqualTo(March2026);
    }

    [Test]
    public async Task Should_PreservePushSentFlag_When_AlertHasPushSent()
    {
        // Arrange
        var alert = DecisionAlert.Create(
            "user-1", "app-uid-001", "Test App", "123 High St", "Permitted", March2026);
        alert.MarkPushSent();

        // Act
        var document = DecisionAlertDocument.FromDomain(alert);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.PushSent).IsTrue();
    }

    [Test]
    public async Task Should_SetTtlTo90Days_When_MappingFromDomain()
    {
        // Arrange
        var alert = DecisionAlert.Create(
            "user-1", "app-uid-001", "Test App", "123 High St", "Permitted", March2026);

        // Act
        var document = DecisionAlertDocument.FromDomain(alert);

        // Assert
        var ninetyDaysInSeconds = (int)TimeSpan.FromDays(90).TotalSeconds;
        await Assert.That(document.Ttl).IsEqualTo(ninetyDaysInSeconds);
    }
}
