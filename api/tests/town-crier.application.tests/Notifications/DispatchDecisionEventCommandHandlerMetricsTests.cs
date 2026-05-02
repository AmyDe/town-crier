using System.Diagnostics.Metrics;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Notifications;
using TownCrier.Application.Tests.DeviceRegistrations;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.SavedApplications;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.SavedApplications;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Notifications;

[NotInParallel]
public sealed class DispatchDecisionEventCommandHandlerMetricsTests
{
    private static readonly DateTimeOffset March2026 = new(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_TagNotificationsCreatedWithDecisionUpdateAndZone_When_DispatchedViaWatchZone()
    {
        // Arrange — Pro user with covering zone
        var harness = new Harness();
        await harness.SeedPaidUserWithZoneAsync("user-1", "zone-1", "device-1");

        var recorded = new List<(long Value, Dictionary<string, string?> Tags)>();
        using var listener = BuildListener("towncrier.notifications.created", recorded);

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(1);
        await Assert.That(recorded[0].Tags["event_type"]).IsEqualTo("DecisionUpdate");
        await Assert.That(recorded[0].Tags["sources"]).IsEqualTo("Zone");
    }

    [Test]
    public async Task Should_TagNotificationsCreatedWithMergedSources_When_UserMatchesViaZoneAndSaved()
    {
        // Arrange — paid user matches via BOTH zone and saved bookmark
        var harness = new Harness();
        await harness.SeedPaidUserWithZoneAsync("user-1", "zone-1", "device-1");
        await harness.SavedApplicationRepo.SaveAsync(
            SavedApplication.Create("user-1", "test-uid-001", March2026),
            CancellationToken.None);

        var recorded = new List<(long Value, Dictionary<string, string?> Tags)>();
        using var listener = BuildListener("towncrier.notifications.created", recorded);

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert — Sources flag is OR-merged; ToString() renders "Zone, Saved"
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Tags["event_type"]).IsEqualTo("DecisionUpdate");
        await Assert.That(recorded[0].Tags["sources"]).IsEqualTo("Zone, Saved");
    }

    [Test]
    public async Task Should_TagNotificationsSentWithEventTypeSourcesAndTier_When_PushFiresForPaidUser()
    {
        // Arrange — Pro user with zone match, push enabled, device registered
        var harness = new Harness();
        await harness.SeedPaidUserWithZoneAsync("user-1", "zone-1", "device-1");

        var recorded = new List<(long Value, Dictionary<string, string?> Tags)>();
        using var listener = BuildListener("towncrier.notifications.sent", recorded);

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert
        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].Value).IsEqualTo(1);
        await Assert.That(recorded[0].Tags["event_type"]).IsEqualTo("DecisionUpdate");
        await Assert.That(recorded[0].Tags["sources"]).IsEqualTo("Zone");
        await Assert.That(recorded[0].Tags["tier"]).IsEqualTo("Pro");
    }

    private static PlanningApplication BuildPermittedApplication()
    {
        return new PlanningApplicationBuilder()
            .WithUid("test-uid-001")
            .WithName("app-001")
            .WithAppState("Permitted")
            .WithCoordinates(51.5074, -0.1278)
            .Build();
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

    private sealed class Harness
    {
        public Harness()
        {
            this.NotificationRepo = new FakeNotificationRepository();
            this.UserProfileRepo = new FakeUserProfileRepository();
            this.SavedApplicationRepo = new FakeSavedApplicationRepository();
            this.WatchZoneRepo = new FakeWatchZoneRepository();
            this.DeviceRepo = new FakeDeviceRegistrationRepository();
            this.PushSender = new SpyPushNotificationSender();
            this.RemoveInvalidHandler = new RemoveInvalidDeviceTokenCommandHandler(this.DeviceRepo);
            this.TimeProvider = new FakeTimeProvider(March2026);

            this.Handler = new DispatchDecisionEventCommandHandler(
                this.NotificationRepo,
                this.UserProfileRepo,
                this.SavedApplicationRepo,
                this.WatchZoneRepo,
                this.DeviceRepo,
                this.PushSender,
                this.RemoveInvalidHandler,
                this.TimeProvider);
        }

        public DispatchDecisionEventCommandHandler Handler { get; }

        public FakeNotificationRepository NotificationRepo { get; }

        public FakeUserProfileRepository UserProfileRepo { get; }

        public FakeSavedApplicationRepository SavedApplicationRepo { get; }

        public FakeWatchZoneRepository WatchZoneRepo { get; }

        public FakeDeviceRegistrationRepository DeviceRepo { get; }

        public SpyPushNotificationSender PushSender { get; }

        public RemoveInvalidDeviceTokenCommandHandler RemoveInvalidHandler { get; }

        public FakeTimeProvider TimeProvider { get; }

        public async Task SeedPaidUserAsync(string userId, string deviceToken)
        {
            var profile = new UserProfileBuilder()
                .WithUserId(userId)
                .WithTier(SubscriptionTier.Pro)
                .Build();
            await this.UserProfileRepo.SaveAsync(profile, CancellationToken.None);

            var device = DeviceRegistration.Create(userId, deviceToken, DevicePlatform.Ios, March2026);
            await this.DeviceRepo.SaveAsync(device, CancellationToken.None);
        }

        public async Task SeedPaidUserWithZoneAsync(
            string userId,
            string zoneId,
            string deviceToken)
        {
            await this.SeedPaidUserAsync(userId, deviceToken);

            var zone = new WatchZoneBuilder()
                .WithId(zoneId)
                .WithUserId(userId)
                .WithCentre(51.5074, -0.1278)
                .WithRadiusMetres(5000)
                .Build();
            this.WatchZoneRepo.Add(zone);
        }
    }
}
