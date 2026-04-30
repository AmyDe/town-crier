using TownCrier.Application.Notifications;
using TownCrier.Application.Tests.DeviceRegistrations;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.SavedApplications;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.SavedApplications;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Notifications;

public sealed class DispatchDecisionEventCommandHandlerTests
{
    private static readonly DateTimeOffset March2026 = new(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_CreateDecisionNotification_When_ApplicationMatchesWatchZoneForPaidUser()
    {
        // Arrange — Pro user has a zone covering the application coordinates
        var harness = new Harness();
        await harness.SeedPaidUserWithZoneAsync("user-1", "zone-1", "device-1");

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert
        await Assert.That(harness.NotificationRepo.All).HasCount().EqualTo(1);
        var notification = harness.NotificationRepo.All[0];
        await Assert.That(notification.UserId).IsEqualTo("user-1");
        await Assert.That(notification.ApplicationUid).IsEqualTo("test-uid-001");
        await Assert.That(notification.WatchZoneId).IsEqualTo("zone-1");
        await Assert.That(notification.EventType).IsEqualTo(NotificationEventType.DecisionUpdate);
        await Assert.That(notification.Sources).IsEqualTo(NotificationSources.Zone);
        await Assert.That(notification.Decision).IsEqualTo("Permitted");
    }

    [Test]
    public async Task Should_CreateSavedDecisionNotification_When_PaidUserHasBookmarkedApplication()
    {
        // Arrange — Pro user has saved this app, not in any zone (no coordinates)
        var harness = new Harness();
        await harness.SeedPaidUserAsync("user-1", "device-1");
        await harness.SavedApplicationRepo.SaveAsync(
            SavedApplication.Create("user-1", "test-uid-001", March2026),
            CancellationToken.None);

        // Act — application has no coords (only saved-bookmark holders)
        var application = new PlanningApplicationBuilder()
            .WithUid("test-uid-001")
            .WithName("app-001")
            .WithAppState("Rejected")
            .Build();

        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(application),
            CancellationToken.None);

        // Assert — Sources=Saved, no WatchZoneId, decision recorded
        await Assert.That(harness.NotificationRepo.All).HasCount().EqualTo(1);
        var notification = harness.NotificationRepo.All[0];
        await Assert.That(notification.UserId).IsEqualTo("user-1");
        await Assert.That(notification.Sources).IsEqualTo(NotificationSources.Saved);
        await Assert.That(notification.WatchZoneId).IsNull();
        await Assert.That(notification.Decision).IsEqualTo("Rejected");
        await Assert.That(notification.EventType).IsEqualTo(NotificationEventType.DecisionUpdate);
    }

    [Test]
    public async Task Should_OrMergeIntoSingleNotification_When_UserMatchesViaBothZoneAndSaved()
    {
        // Arrange — Pro user has BOTH a covering zone AND a saved bookmark
        var harness = new Harness();
        await harness.SeedPaidUserWithZoneAsync("user-1", "zone-1", "device-1");
        await harness.SavedApplicationRepo.SaveAsync(
            SavedApplication.Create("user-1", "test-uid-001", March2026),
            CancellationToken.None);

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert — exactly ONE row with Sources flags OR-merged
        await Assert.That(harness.NotificationRepo.All).HasCount().EqualTo(1);
        var notification = harness.NotificationRepo.All[0];
        await Assert.That(notification.Sources).IsEqualTo(NotificationSources.Zone | NotificationSources.Saved);
        await Assert.That(notification.WatchZoneId).IsEqualTo("zone-1");
    }

    [Test]
    public async Task Should_NotCreateDuplicate_When_DecisionUpdateAlreadyExistsForUser()
    {
        // Arrange — Pro user matches via zone, but a DecisionUpdate row already
        // exists for (user, applicationUid). Re-dispatch must be a no-op.
        var harness = new Harness();
        await harness.SeedPaidUserWithZoneAsync("user-1", "zone-1", "device-1");

        var existing = Notification.Create(
            userId: "user-1",
            applicationUid: "test-uid-001",
            applicationName: "app-001",
            watchZoneId: "zone-1",
            applicationAddress: "1 High St",
            applicationDescription: "Extension",
            applicationType: "Householder",
            authorityId: 1,
            now: March2026,
            decision: "Permitted",
            eventType: NotificationEventType.DecisionUpdate,
            sources: NotificationSources.Zone);
        harness.NotificationRepo.Seed(existing);

        // Act — re-dispatch the same decision event
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert — still exactly one row
        await Assert.That(harness.NotificationRepo.All).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_CreateDecisionUpdate_When_NewApplicationAlreadyExistsForSameUid()
    {
        // Arrange — user already has a NewApplication row for this uid; the
        // DecisionUpdate must NOT collide (dedup keys on eventType too).
        var harness = new Harness();
        await harness.SeedPaidUserWithZoneAsync("user-1", "zone-1", "device-1");

        var existing = Notification.Create(
            userId: "user-1",
            applicationUid: "test-uid-001",
            applicationName: "app-001",
            watchZoneId: "zone-1",
            applicationAddress: "1 High St",
            applicationDescription: "Extension",
            applicationType: "Householder",
            authorityId: 1,
            now: March2026,
            eventType: NotificationEventType.NewApplication);
        harness.NotificationRepo.Seed(existing);

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert — both rows exist
        await Assert.That(harness.NotificationRepo.All).HasCount().EqualTo(2);
        await Assert.That(harness.NotificationRepo.All.Count(n => n.EventType == NotificationEventType.NewApplication)).IsEqualTo(1);
        await Assert.That(harness.NotificationRepo.All.Count(n => n.EventType == NotificationEventType.DecisionUpdate)).IsEqualTo(1);
    }

    private static PlanningApplication BuildPermittedApplication(
        string uid = "test-uid-001",
        string name = "app-001",
        string appState = "Permitted")
    {
        return new PlanningApplicationBuilder()
            .WithUid(uid)
            .WithName(name)
            .WithAppState(appState)
            .WithCoordinates(51.5074, -0.1278)
            .Build();
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
            this.TimeProvider = new FakeTimeProvider(March2026);

            this.Handler = new DispatchDecisionEventCommandHandler(
                this.NotificationRepo,
                this.UserProfileRepo,
                this.SavedApplicationRepo,
                this.WatchZoneRepo,
                this.DeviceRepo,
                this.PushSender,
                this.TimeProvider);
        }

        public DispatchDecisionEventCommandHandler Handler { get; }

        public FakeNotificationRepository NotificationRepo { get; }

        public FakeUserProfileRepository UserProfileRepo { get; }

        public FakeSavedApplicationRepository SavedApplicationRepo { get; }

        public FakeWatchZoneRepository WatchZoneRepo { get; }

        public FakeDeviceRegistrationRepository DeviceRepo { get; }

        public SpyPushNotificationSender PushSender { get; }

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
