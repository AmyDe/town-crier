using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;
using TownCrier.Infrastructure.Notifications;

namespace TownCrier.Infrastructure.Tests.Notifications;

public sealed class AcsEmailDigestCopyTests
{
    [Test]
    public async Task Should_NotIncludeWeeklyCadenceLanguage_When_BuildingDigestSubject()
    {
        // Arrange
        var totalCount = 5;

        // Act
        var subject = AcsEmailSender.BuildDigestSubject(totalCount);

        // Assert
        await Assert.That(subject).DoesNotContain("weekly");
        await Assert.That(subject).DoesNotContain("Weekly");
        await Assert.That(subject).DoesNotContain("hourly");
        await Assert.That(subject).DoesNotContain("Hourly");
        await Assert.That(subject).DoesNotContain("daily");
        await Assert.That(subject).DoesNotContain("Daily");
    }

    [Test]
    public async Task Should_IncludeApplicationCountInSubject_When_BuildingDigestSubject()
    {
        // Arrange
        var totalCount = 7;

        // Act
        var subject = AcsEmailSender.BuildDigestSubject(totalCount);

        // Assert
        await Assert.That(subject).Contains("7");
    }

    [Test]
    public async Task Should_UseEvergreenSubjectCopy_When_BuildingDigestSubject()
    {
        // Arrange
        var totalCount = 3;

        // Act
        var subject = AcsEmailSender.BuildDigestSubject(totalCount);

        // Assert
        await Assert.That(subject).IsEqualTo("Planning update — 3 new applications near you");
    }

    [Test]
    public async Task Should_NotIncludeWeeklyCadenceLanguage_When_BuildingDigestHtml()
    {
        // Arrange
        var notification = new NotificationBuilder().Build();
        var digests = new List<WatchZoneDigest>
        {
            new("Home", new List<Notification> { notification }),
        };

        // Act
        var html = AcsEmailSender.BuildDigestHtml(digests, totalCount: 1);

        // Assert
        await Assert.That(html).DoesNotContain("Weekly");
        await Assert.That(html).DoesNotContain("weekly");
        await Assert.That(html).DoesNotContain("Hourly");
        await Assert.That(html).DoesNotContain("hourly");
        await Assert.That(html).DoesNotContain("Daily");
        await Assert.That(html).DoesNotContain("daily");
        await Assert.That(html).DoesNotContain("this week");
    }

    [Test]
    public async Task Should_UseLivePlanningUpdateSubtitle_When_BuildingDigestHtml()
    {
        // Arrange
        var notification = new NotificationBuilder().Build();
        var digests = new List<WatchZoneDigest>
        {
            new("Home", new List<Notification> { notification }),
        };

        // Act
        var html = AcsEmailSender.BuildDigestHtml(digests, totalCount: 1);

        // Assert
        await Assert.That(html).Contains("Live Planning Update");
    }

    [Test]
    public async Task Should_RenderApprovedBadge_When_NotificationIsPermittedDecision()
    {
        // Arrange
        var decision = new NotificationBuilder()
            .WithApplicationAddress("4 High Street")
            .WithEventType(NotificationEventType.DecisionUpdate)
            .WithDecision("Permitted")
            .Build();
        var digests = new List<WatchZoneDigest>
        {
            new("Home", new List<Notification> { decision }),
        };

        // Act
        var html = AcsEmailSender.BuildDigestHtml(digests, totalCount: 1);

        // Assert
        await Assert.That(html).Contains("[Approved]");
    }

    [Test]
    public async Task Should_RenderRefusedBadge_When_NotificationIsRejectedDecision()
    {
        // Arrange
        var decision = new NotificationBuilder()
            .WithEventType(NotificationEventType.DecisionUpdate)
            .WithDecision("Rejected")
            .Build();
        var digests = new List<WatchZoneDigest>
        {
            new("Home", new List<Notification> { decision }),
        };

        // Act
        var html = AcsEmailSender.BuildDigestHtml(digests, totalCount: 1);

        // Assert
        await Assert.That(html).Contains("[Refused]");
    }

    [Test]
    public async Task Should_RenderApprovedWithConditionsBadge_When_NotificationIsConditionsDecision()
    {
        // Arrange
        var decision = new NotificationBuilder()
            .WithEventType(NotificationEventType.DecisionUpdate)
            .WithDecision("Conditions")
            .Build();
        var digests = new List<WatchZoneDigest>
        {
            new("Home", new List<Notification> { decision }),
        };

        // Act
        var html = AcsEmailSender.BuildDigestHtml(digests, totalCount: 1);

        // Assert
        await Assert.That(html).Contains("[Approved with conditions]");
    }

    [Test]
    public async Task Should_RenderRefusalAppealedBadge_When_NotificationIsAppealedDecision()
    {
        // Arrange
        var decision = new NotificationBuilder()
            .WithEventType(NotificationEventType.DecisionUpdate)
            .WithDecision("Appealed")
            .Build();
        var digests = new List<WatchZoneDigest>
        {
            new("Home", new List<Notification> { decision }),
        };

        // Act
        var html = AcsEmailSender.BuildDigestHtml(digests, totalCount: 1);

        // Assert
        await Assert.That(html).Contains("[Refusal appealed]");
    }

    [Test]
    public async Task Should_RenderSavedIndicator_When_NotificationIsZonePlusSaved()
    {
        // Arrange
        var notification = new NotificationBuilder()
            .WithSources(NotificationSources.Zone | NotificationSources.Saved)
            .Build();
        var digests = new List<WatchZoneDigest>
        {
            new("Home", new List<Notification> { notification }),
        };

        // Act
        var html = AcsEmailSender.BuildDigestHtml(digests, totalCount: 1);

        // Assert
        await Assert.That(html).Contains("saved");
    }

    [Test]
    public async Task Should_NotRenderSavedIndicator_When_NotificationIsZoneOnly()
    {
        // Arrange
        var notification = new NotificationBuilder()
            .WithApplicationDescription("Two-storey rear extension")
            .WithSources(NotificationSources.Zone)
            .Build();
        var digests = new List<WatchZoneDigest>
        {
            new("Home", new List<Notification> { notification }),
        };

        // Act
        var html = AcsEmailSender.BuildDigestHtml(digests, totalCount: 1);

        // Assert -- avoid false positives by checking for the indicator-class marker
        await Assert.That(html).DoesNotContain("data-saved-indicator");
    }

    [Test]
    public async Task Should_NotRenderDecisionBadge_When_NotificationIsNewApplication()
    {
        // Arrange
        var newApp = new NotificationBuilder()
            .WithEventType(NotificationEventType.NewApplication)
            .Build();
        var digests = new List<WatchZoneDigest>
        {
            new("Home", new List<Notification> { newApp }),
        };

        // Act
        var html = AcsEmailSender.BuildDigestHtml(digests, totalCount: 1);

        // Assert
        await Assert.That(html).DoesNotContain("[Approved]");
        await Assert.That(html).DoesNotContain("[Refused]");
    }
}
