using System.Diagnostics.Metrics;
using TownCrier.Application.Notifications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Notifications;

[NotInParallel]
public sealed class GenerateHourlyDigestsCommandHandlerMetricsTests
{
    private static readonly DateTimeOffset March2026 = new(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_EmitDigestRowsPerNotification_When_HourlyDigestSent()
    {
        // Arrange — Personal user (eligible for hourly), one zone, three new-app rows + one decision row
        var notificationRepo = new FakeNotificationRepository();
        var userProfileRepo = new FakeUserProfileRepository();
        var emailSender = new SpyEmailSender();
        var watchZoneRepo = new FakeWatchZoneRepository();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .WithTier(SubscriptionTier.Personal)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("Home")
            .Build();
        await watchZoneRepo.SaveAsync(zone, CancellationToken.None);

        notificationRepo.Seed(BuildNotification("user-1", "zone-1", "uid-1", NotificationEventType.NewApplication));
        notificationRepo.Seed(BuildNotification("user-1", "zone-1", "uid-2", NotificationEventType.NewApplication));
        notificationRepo.Seed(BuildNotification("user-1", "zone-1", "uid-3", NotificationEventType.NewApplication));
        notificationRepo.Seed(BuildNotification("user-1", "zone-1", "uid-4", NotificationEventType.DecisionUpdate));

        var handler = new GenerateHourlyDigestsCommandHandler(
            notificationRepo, userProfileRepo, emailSender, watchZoneRepo);

        var recorded = new List<(long Value, Dictionary<string, string?> Tags)>();
        using var listener = BuildListener("towncrier.digest.rows_emitted", recorded);

        // Act
        await handler.HandleAsync(new GenerateHourlyDigestsCommand(), CancellationToken.None);

        // Assert — one emission per notification, all tagged cadence=hourly with the
        // appropriate event_type
        await Assert.That(recorded).HasCount().EqualTo(4);
        await Assert.That(recorded.TrueForAll(r => r.Value == 1)).IsTrue();
        await Assert.That(recorded.TrueForAll(r => r.Tags["cadence"] == "hourly")).IsTrue();
        await Assert.That(recorded.Count(r => r.Tags["event_type"] == "NewApplication")).IsEqualTo(3);
        await Assert.That(recorded.Count(r => r.Tags["event_type"] == "DecisionUpdate")).IsEqualTo(1);
    }

    private static Notification BuildNotification(
        string userId,
        string zoneId,
        string uid,
        NotificationEventType eventType)
    {
        return Notification.Create(
            userId: userId,
            applicationUid: uid,
            applicationName: $"name-{uid}",
            watchZoneId: zoneId,
            applicationAddress: "1 High St",
            applicationDescription: "Extension",
            applicationType: "Householder",
            authorityId: 1,
            now: March2026,
            decision: eventType == NotificationEventType.DecisionUpdate ? "Permitted" : null,
            eventType: eventType,
            sources: NotificationSources.Zone);
    }

    private static MeterListener BuildListener(
        string instrumentName,
        List<(long Value, Dictionary<string, string?> Tags)> recorded)
    {
        var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, l) =>
        {
            if (instrument.Name == instrumentName)
            {
                l.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            var dict = new Dictionary<string, string?>(StringComparer.Ordinal);
            foreach (var tag in tags)
            {
                dict[tag.Key] = tag.Value?.ToString();
            }

            recorded.Add((measurement, dict));
        });
        listener.Start();
        return listener;
    }
}
