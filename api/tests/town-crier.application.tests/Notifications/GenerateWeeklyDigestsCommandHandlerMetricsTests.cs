using System.Diagnostics.Metrics;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Notifications;
using TownCrier.Application.Tests.NotificationState;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.UserProfiles;
using FakeDeviceRegistrationRepository = TownCrier.Application.Tests.DeviceRegistrations.FakeDeviceRegistrationRepository;
using FakeTimeProvider = TownCrier.Application.Tests.DeviceRegistrations.FakeTimeProvider;

namespace TownCrier.Application.Tests.Notifications;

[NotInParallel]
public sealed class GenerateWeeklyDigestsCommandHandlerMetricsTests
{
    // Monday 2026-03-16 at 08:00 UTC — same anchor as the existing weekly tests
    private static readonly DateTimeOffset MondayMarch2026 = new(2026, 3, 16, 8, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_EmitDigestRowsPerNotification_When_WeeklyDigestSent()
    {
        // Arrange — Pro user (eligible for weekly push + email), digest day=Monday,
        // mix of new-app and decision-update rows from the past week.
        var notificationRepo = new FakeNotificationRepository();
        var notificationStateRepo = new FakeNotificationStateRepository();
        var userProfileRepo = new FakeUserProfileRepository();
        var deviceRepo = new FakeDeviceRegistrationRepository();
        var pushSender = new SpyPushNotificationSender();
        var emailSender = new SpyEmailSender();
        var watchZoneRepo = new FakeWatchZoneRepository();
        var timeProvider = new FakeTimeProvider(MondayMarch2026);

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("user-1@example.com")
            .WithTier(SubscriptionTier.Pro)
            .WithDigestDay(DayOfWeek.Monday)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var createdAt = MondayMarch2026.AddDays(-2);
        notificationRepo.Seed(BuildNotification("user-1", "zone-1", "uid-1", NotificationEventType.NewApplication, createdAt));
        notificationRepo.Seed(BuildNotification("user-1", "zone-1", "uid-2", NotificationEventType.NewApplication, createdAt));
        notificationRepo.Seed(BuildNotification("user-1", "zone-1", "uid-3", NotificationEventType.DecisionUpdate, createdAt));

        var removeInvalidHandler = new RemoveInvalidDeviceTokenCommandHandler(deviceRepo);
        var handler = new GenerateWeeklyDigestsCommandHandler(
            userProfileRepo,
            notificationRepo,
            notificationStateRepo,
            deviceRepo,
            pushSender,
            removeInvalidHandler,
            emailSender,
            watchZoneRepo,
            timeProvider);

        var recorded = new List<(long Value, Dictionary<string, string?> Tags)>();
        using var listener = BuildListener("towncrier.digest.rows_emitted", recorded);

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert — one emission per row, tagged cadence=weekly with the right event_type
        await Assert.That(recorded).HasCount().EqualTo(3);
        await Assert.That(recorded.TrueForAll(r => r.Value == 1)).IsTrue();
        await Assert.That(recorded.TrueForAll(r => r.Tags["cadence"] == "weekly")).IsTrue();
        await Assert.That(recorded.Count(r => r.Tags["event_type"] == "NewApplication")).IsEqualTo(2);
        await Assert.That(recorded.Count(r => r.Tags["event_type"] == "DecisionUpdate")).IsEqualTo(1);
    }

    private static Notification BuildNotification(
        string userId,
        string zoneId,
        string uid,
        NotificationEventType eventType,
        DateTimeOffset createdAt)
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
            now: createdAt,
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
