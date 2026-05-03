using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;
using TownCrier.Infrastructure.Notifications;

namespace TownCrier.Infrastructure.Tests.Notifications;

public sealed class AcsEmailSenderTests
{
    [Test]
    public async Task Should_WrapEachNotificationCardLineInApplicationDetailLink_When_BuildingDigestHtml()
    {
        // Arrange — a watch-zone notification for a known PlanIt UID.
        var notification = new NotificationBuilder()
            .WithApplicationUid("simple-uid-001")
            .Build();
        var digests = new List<WatchZoneDigest>
        {
            new("Home", new List<Notification> { notification }),
        };

        // Act
        var html = AcsEmailSender.BuildDigestHtml(digests, Array.Empty<Notification>(), totalCount: 1);

        // Assert — every card carries an href to the application detail page so
        // Universal Links can intercept on iOS and the web app handles it
        // otherwise. We expect one href per line of the card (3 lines).
        const string expectedHref =
            "<a href=\"https://towncrierapp.uk/applications/simple-uid-001\" style=\"text-decoration:none;color:inherit;\">";
        await Assert.That(CountOccurrences(html, expectedHref)).IsEqualTo(3);
    }

    private static int CountOccurrences(string haystack, string needle)
    {
        if (needle.Length == 0)
        {
            return 0;
        }

        var count = 0;
        var index = 0;
        while ((index = haystack.IndexOf(needle, index, StringComparison.Ordinal)) >= 0)
        {
            count++;
            index += needle.Length;
        }

        return count;
    }
}
