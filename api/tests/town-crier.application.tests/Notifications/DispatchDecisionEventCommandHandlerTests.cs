using TownCrier.Application.DeviceRegistrations;
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

    [Test]
    public async Task Should_SendPushAndMarkSent_When_PaidUserHasZoneDecisionPushOn()
    {
        // Arrange — Pro user with zone DecisionPush=true (default), device registered
        var harness = new Harness();
        await harness.SeedPaidUserWithZoneAsync("user-1", "zone-1", "device-1");

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert
        await Assert.That(harness.PushSender.Sent).HasCount().EqualTo(1);
        await Assert.That(harness.PushSender.Sent[0].Notification.ApplicationUid).IsEqualTo("test-uid-001");
        await Assert.That(harness.PushSender.Sent[0].Devices).HasCount().EqualTo(1);
        await Assert.That(harness.NotificationRepo.All[0].PushSent).IsTrue();
    }

    [Test]
    public async Task Should_RecordButNotPush_When_UserOnFreeTier()
    {
        // Arrange — Free tier matches via zone, has device + push enabled
        var harness = new Harness();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .Build(); // Free by default
        await harness.UserProfileRepo.SaveAsync(profile, CancellationToken.None);

        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithCentre(51.5074, -0.1278)
            .Build();
        harness.WatchZoneRepo.Add(zone);

        var device = DeviceRegistration.Create("user-1", "device-1", DevicePlatform.Ios, March2026);
        await harness.DeviceRepo.SaveAsync(device, CancellationToken.None);

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert — row written for the digest, but no push
        await Assert.That(harness.NotificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(harness.NotificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(harness.PushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_RecordButNotPush_When_MasterPushDisabled()
    {
        // Arrange — paid user, zone matches, but the master PushEnabled toggle is off
        var harness = new Harness();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .WithPushEnabled(false)
            .Build();
        await harness.UserProfileRepo.SaveAsync(profile, CancellationToken.None);

        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithCentre(51.5074, -0.1278)
            .Build();
        harness.WatchZoneRepo.Add(zone);

        var device = DeviceRegistration.Create("user-1", "device-1", DevicePlatform.Ios, March2026);
        await harness.DeviceRepo.SaveAsync(device, CancellationToken.None);

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert
        await Assert.That(harness.NotificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(harness.NotificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(harness.PushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_RecordButNotPush_When_ZoneDecisionPushDisabled()
    {
        // Arrange — paid user with zone DecisionPush=false, no saved match
        var harness = new Harness();
        await harness.SeedPaidUserWithZoneAsync("user-1", "zone-1", "device-1");

        var profile = await harness.UserProfileRepo.GetByUserIdAsync("user-1", CancellationToken.None);
        profile!.SetZonePreferences(
            "zone-1",
            new ZoneNotificationPreferences(
                NewApplicationPush: true,
                NewApplicationEmail: true,
                DecisionPush: false,
                DecisionEmail: true));
        await harness.UserProfileRepo.SaveAsync(profile, CancellationToken.None);

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert
        await Assert.That(harness.NotificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(harness.NotificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(harness.PushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_RecordButNotPush_When_SavedDecisionPushDisabled()
    {
        // Arrange — paid user with saved bookmark, profile-level
        // SavedDecisionPush=false; not a zone match either.
        var harness = new Harness();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        profile.UpdatePreferences(profile.NotificationPreferences with { SavedDecisionPush = false });
        await harness.UserProfileRepo.SaveAsync(profile, CancellationToken.None);

        await harness.SavedApplicationRepo.SaveAsync(
            SavedApplication.Create("user-1", "test-uid-001", March2026),
            CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-1", DevicePlatform.Ios, March2026);
        await harness.DeviceRepo.SaveAsync(device, CancellationToken.None);

        // Act — no coordinates, so saved-only path
        var application = new PlanningApplicationBuilder()
            .WithUid("test-uid-001")
            .WithName("app-001")
            .WithAppState("Permitted")
            .Build();

        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(application),
            CancellationToken.None);

        // Assert
        await Assert.That(harness.NotificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(harness.NotificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(harness.PushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_PushOnce_When_BothSourcesMatchAndEitherDecisionPushIsOn()
    {
        // Arrange — paid user matches via Zone (DecisionPush=false) AND Saved
        // (SavedDecisionPush=true). OR-merge means push fires.
        var harness = new Harness();
        await harness.SeedPaidUserWithZoneAsync("user-1", "zone-1", "device-1");

        var profile = await harness.UserProfileRepo.GetByUserIdAsync("user-1", CancellationToken.None);
        profile!.SetZonePreferences(
            "zone-1",
            new ZoneNotificationPreferences(
                NewApplicationPush: false,
                NewApplicationEmail: false,
                DecisionPush: false,
                DecisionEmail: false));
        await harness.UserProfileRepo.SaveAsync(profile, CancellationToken.None);

        await harness.SavedApplicationRepo.SaveAsync(
            SavedApplication.Create("user-1", "test-uid-001", March2026),
            CancellationToken.None);

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert — exactly one notification, exactly one push
        await Assert.That(harness.NotificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(harness.PushSender.Sent).HasCount().EqualTo(1);
        await Assert.That(harness.NotificationRepo.All[0].PushSent).IsTrue();
        await Assert.That(harness.NotificationRepo.All[0].Sources)
            .IsEqualTo(NotificationSources.Zone | NotificationSources.Saved);
    }

    [Test]
    public async Task Should_NotCreateNotification_When_UserProfileMissing()
    {
        // Arrange — saved bookmark held by an unknown user (race against profile delete)
        var harness = new Harness();
        await harness.SavedApplicationRepo.SaveAsync(
            SavedApplication.Create("ghost-user", "test-uid-001", March2026),
            CancellationToken.None);

        var application = new PlanningApplicationBuilder()
            .WithUid("test-uid-001")
            .WithAppState("Permitted")
            .Build();

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(application),
            CancellationToken.None);

        // Assert
        await Assert.That(harness.NotificationRepo.All).HasCount().EqualTo(0);
        await Assert.That(harness.PushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_RecordWithoutPushing_When_NoRegisteredDevices()
    {
        // Arrange — paid user, zone match, push opted in, but no devices
        var harness = new Harness();
        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await harness.UserProfileRepo.SaveAsync(profile, CancellationToken.None);

        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithCentre(51.5074, -0.1278)
            .Build();
        harness.WatchZoneRepo.Add(zone);

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert — row written, no push (no devices)
        await Assert.That(harness.NotificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(harness.NotificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(harness.PushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_FanOutOneNotificationPerUser_When_MultipleUsersMatch()
    {
        // Arrange — two paid users, each with a zone covering the application
        var harness = new Harness();
        await harness.SeedPaidUserWithZoneAsync("user-1", "zone-1", "device-1");
        await harness.SeedPaidUserWithZoneAsync("user-2", "zone-2", "device-2");

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert
        await Assert.That(harness.NotificationRepo.All).HasCount().EqualTo(2);
        await Assert.That(harness.PushSender.Sent).HasCount().EqualTo(2);
        await Assert.That(harness.NotificationRepo.All.Any(n => n.UserId == "user-1")).IsTrue();
        await Assert.That(harness.NotificationRepo.All.Any(n => n.UserId == "user-2")).IsTrue();
    }

    [Test]
    public async Task Should_PruneInvalidTokens_When_DecisionSenderReportsRejections()
    {
        // Arrange — Pro user with two devices; sender reports the second invalid.
        var harness = new Harness();
        await harness.SeedPaidUserWithZoneAsync("user-1", "zone-1", "good-token");

        // Add a second device to the same user
        var staleDevice = DeviceRegistration.Create("user-1", "stale-token", DevicePlatform.Ios, March2026);
        await harness.DeviceRepo.SaveAsync(staleDevice, CancellationToken.None);

        harness.PushSender.NextInvalidTokens = new[] { "stale-token" };

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert — exactly one removal, against the stale token
        await Assert.That(harness.DeviceRepo.DeletedTokens).HasCount().EqualTo(1);
        await Assert.That(harness.DeviceRepo.DeletedTokens[0]).IsEqualTo("stale-token");
        await Assert.That(harness.DeviceRepo.GetByToken("good-token")).IsNotNull();
        await Assert.That(harness.DeviceRepo.GetByToken("stale-token")).IsNull();
    }

    [Test]
    public async Task Should_NotPruneAnyTokens_When_DecisionSenderReportsNoRejections()
    {
        // Arrange — happy path on the decision dispatch path; no rejections.
        var harness = new Harness();
        await harness.SeedPaidUserWithZoneAsync("user-1", "zone-1", "good-token");

        // Default NextInvalidTokens is empty.

        // Act
        await harness.Handler.HandleAsync(
            new DispatchDecisionEventCommand(BuildPermittedApplication()),
            CancellationToken.None);

        // Assert — push sent, no removal calls
        await Assert.That(harness.PushSender.Sent).HasCount().EqualTo(1);
        await Assert.That(harness.DeviceRepo.DeletedTokens).HasCount().EqualTo(0);
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
