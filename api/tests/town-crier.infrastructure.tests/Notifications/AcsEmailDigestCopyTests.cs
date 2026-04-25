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
}
