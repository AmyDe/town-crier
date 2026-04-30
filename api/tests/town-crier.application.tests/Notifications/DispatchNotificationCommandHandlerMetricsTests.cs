using System.Diagnostics.Metrics;
using TownCrier.Application.Notifications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.UserProfiles;
using FakeDeviceRegistrationRepository = TownCrier.Application.Tests.DeviceRegistrations.FakeDeviceRegistrationRepository;

namespace TownCrier.Application.Tests.Notifications;

[NotInParallel]
public sealed class DispatchNotificationCommandHandlerMetricsTests
{
    private static readonly DateTimeOffset March2026 = new(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_TagNotificationsCreatedWithEventTypeAndSources_When_NewApplicationDispatched()
    {
        // Arrange — paid user with zone match, push enabled
        var (handler, _, userProfileRepo, _, deviceRepo) = CreateHandler();
        await SeedPaidUserWithDevice(userProfileRepo, deviceRepo);

        var recorded = new List<(long Value, Dictionary<string, string?> Tags)>();
        using var listener = BuildListener("towncrier.notifications.created", recorded);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert — single emission, tagged with NewApplication + Zone
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(1);
        await Assert.That(recorded[0].Tags["event_type"]).IsEqualTo("NewApplication");
        await Assert.That(recorded[0].Tags["sources"]).IsEqualTo("Zone");
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

    private static (DispatchNotificationCommandHandler Handler,
        FakeNotificationRepository NotificationRepo,
        FakeUserProfileRepository UserProfileRepo,
        SpyPushNotificationSender PushSender,
        FakeDeviceRegistrationRepository DeviceRepo) CreateHandler()
    {
        var notificationRepo = new FakeNotificationRepository();
        var userProfileRepo = new FakeUserProfileRepository();
        var deviceRepo = new FakeDeviceRegistrationRepository();
        var pushSender = new SpyPushNotificationSender();
        var tp = new FakeTimeProvider(March2026);

        var handler = new DispatchNotificationCommandHandler(
            notificationRepo, userProfileRepo, deviceRepo, pushSender, tp);

        return (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo);
    }

    private static async Task SeedPaidUserWithDevice(
        FakeUserProfileRepository userProfileRepo,
        FakeDeviceRegistrationRepository deviceRepo,
        string userId = "user-1")
    {
        var profile = new UserProfileBuilder()
            .WithUserId(userId)
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create(userId, "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);
    }

    private static DispatchNotificationCommand CreateCommand(
        string applicationName = "app-001",
        string applicationUid = "test-uid-001")
    {
        var application = new PlanningApplicationBuilder()
            .WithUid(applicationUid)
            .WithName(applicationName)
            .WithCoordinates(51.5074, -0.1278)
            .Build();

        var zone = new WatchZoneBuilder()
            .WithUserId("user-1")
            .WithId("zone-1")
            .Build();

        return new DispatchNotificationCommand(application, zone);
    }
}
