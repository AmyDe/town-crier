using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Notifications;
using TownCrier.Application.Tests.NotificationState;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.NotificationState;
using TownCrier.Domain.UserProfiles;
using FakeDeviceRegistrationRepository = TownCrier.Application.Tests.DeviceRegistrations.FakeDeviceRegistrationRepository;

namespace TownCrier.Application.Tests.Notifications;

public sealed class DispatchNotificationCommandHandlerTests
{
    private static readonly DateTimeOffset March2026 = new(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_CreateNotificationRecord_When_ApplicationMatchesWatchZone()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, _, deviceRepo, _) = CreateHandler();
        await SeedPaidUserWithDevice(userProfileRepo, deviceRepo);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(notificationRepo.All[0].UserId).IsEqualTo("user-1");
        await Assert.That(notificationRepo.All[0].ApplicationName).IsEqualTo("app-001");
        await Assert.That(notificationRepo.All[0].WatchZoneId).IsEqualTo("zone-1");
    }

    [Test]
    public async Task Should_SendPushNotification_When_PaidUserHasRegisteredDevice()
    {
        // Arrange
        var (handler, _, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();
        await SeedPaidUserWithDevice(userProfileRepo, deviceRepo);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.Sent).HasCount().EqualTo(1);
        await Assert.That(pushSender.Sent[0].Devices).HasCount().EqualTo(1);
        await Assert.That(pushSender.Sent[0].Notification.ApplicationName).IsEqualTo("app-001");
    }

    [Test]
    public async Task Should_MarkPushSent_When_NotificationDispatchedToPaidUser()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, _, deviceRepo, _) = CreateHandler();
        await SeedPaidUserWithDevice(userProfileRepo, deviceRepo);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All[0].PushSent).IsTrue();
    }

    [Test]
    public async Task Should_NotSendDuplicateNotification_When_SameApplicationAndUserAndEventType()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();
        await SeedPaidUserWithDevice(userProfileRepo, deviceRepo);

        var command = CreateCommand();

        // Act
        await handler.HandleAsync(command, CancellationToken.None);
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(pushSender.Sent).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_CreateNewApplicationNotification_When_DecisionUpdateAlreadyExistsForSameUserAndApplication()
    {
        // Arrange — DecisionUpdate already persisted for the same user + applicationUid.
        // Dedup key is (userId, applicationUid, eventType) so a NewApplication dispatch
        // must NOT collide with the pre-existing DecisionUpdate row.
        var (handler, notificationRepo, userProfileRepo, _, deviceRepo, _) = CreateHandler();
        await SeedPaidUserWithDevice(userProfileRepo, deviceRepo);

        var existingDecisionUpdate = Notification.Create(
            userId: "user-1",
            applicationUid: "test-uid-001",
            applicationName: "app-001",
            watchZoneId: "zone-1",
            applicationAddress: "1 High St",
            applicationDescription: "Extension",
            applicationType: "Householder",
            authorityId: 42,
            now: March2026,
            decision: "Permitted",
            eventType: NotificationEventType.DecisionUpdate);
        notificationRepo.Seed(existingDecisionUpdate);

        // Act — dispatch a NewApplication for the same user + applicationUid
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert — both rows exist, no dedup collision across event types
        await Assert.That(notificationRepo.All).HasCount().EqualTo(2);
        await Assert.That(notificationRepo.All.Count(n => n.EventType == NotificationEventType.NewApplication)).IsEqualTo(1);
        await Assert.That(notificationRepo.All.Count(n => n.EventType == NotificationEventType.DecisionUpdate)).IsEqualTo(1);
    }

    [Test]
    public async Task Should_NotCreateDuplicate_When_NewApplicationAlreadyExistsWithSameUidAndEventType()
    {
        // Arrange — pre-existing NewApplication for the same applicationUid; subsequent
        // NewApplication dispatch must dedup (same eventType + same uid + same user).
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();
        await SeedPaidUserWithDevice(userProfileRepo, deviceRepo);

        var existing = Notification.Create(
            userId: "user-1",
            applicationUid: "test-uid-001",
            applicationName: "app-001",
            watchZoneId: "zone-1",
            applicationAddress: "1 High St",
            applicationDescription: "Extension",
            applicationType: "Householder",
            authorityId: 42,
            now: March2026,
            eventType: NotificationEventType.NewApplication);
        notificationRepo.Seed(existing);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert — dedup hits, no extra row, no push
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_RecordNotificationButNotPush_When_PushDisabled()
    {
        // Arrange — paid tier with push toggled off; row written, no push
        var (handler, notificationRepo, userProfileRepo, pushSender, _, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .WithPushEnabled(false)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(notificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_RecordButNotPush_When_UserOnFreeTier()
    {
        // Arrange — free tier (default) gets weekly digest only, no instant push
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();
        await SeedFreeUserWithDevice(userProfileRepo, deviceRepo);

        // Act — dispatch 10 notifications well past any historical cap
        for (var i = 0; i < 10; i++)
        {
            await handler.HandleAsync(
                CreateCommand($"app-{i:D3}", $"uid-{i:D3}"), CancellationToken.None);
        }

        // Assert — all rows persisted (digest will pick them up) but no pushes sent
        await Assert.That(notificationRepo.All).HasCount().EqualTo(10);
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);

        for (var i = 0; i < 10; i++)
        {
            await Assert.That(notificationRepo.All[i].PushSent).IsFalse();
        }
    }

    [Test]
    public async Task Should_AllowUnlimitedNotifications_When_ProTier()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        for (var i = 0; i < 10; i++)
        {
            await handler.HandleAsync(
                CreateCommand($"app-{i:D3}", $"uid-{i:D3}"), CancellationToken.None);
        }

        // Assert — all 10 pushed
        await Assert.That(notificationRepo.All).HasCount().EqualTo(10);
        await Assert.That(pushSender.Sent).HasCount().EqualTo(10);

        for (var i = 0; i < 10; i++)
        {
            await Assert.That(notificationRepo.All[i].PushSent).IsTrue();
        }
    }

    [Test]
    public async Task Should_NotCreateNotification_When_UserProfileNotFound()
    {
        // Arrange — no user profile seeded
        var (handler, notificationRepo, _, pushSender, _, _) = CreateHandler();

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(0);
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotSendPush_When_NoRegisteredDevices()
    {
        // Arrange — paid user with no devices
        var (handler, notificationRepo, userProfileRepo, pushSender, _, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(notificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_RecordButNotPush_When_ZoneNewApplicationPushDisabled()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        profile.SetZonePreferences(
            "zone-1",
            new ZoneNotificationPreferences(
                NewApplicationPush: false,
                NewApplicationEmail: false,
                DecisionPush: false,
                DecisionEmail: false));
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert — notification recorded but push not sent
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(notificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_SendPush_When_ZoneNewApplicationPushEnabled()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        profile.SetZonePreferences(
            "zone-1",
            new ZoneNotificationPreferences(
                NewApplicationPush: true,
                NewApplicationEmail: false,
                DecisionPush: false,
                DecisionEmail: false));
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(notificationRepo.All[0].PushSent).IsTrue();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_PruneInvalidTokens_When_SenderReportsRejections()
    {
        // Arrange — Pro user with two devices; sender reports the second as invalid.
        // Handler must dispatch RemoveInvalidDeviceTokenCommand for each rejected
        // token (one call per token, exactly).
        var (handler, _, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var goodDevice = DeviceRegistration.Create("user-1", "good-token", DevicePlatform.Ios, March2026);
        var staleDevice = DeviceRegistration.Create("user-1", "stale-token", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(goodDevice, CancellationToken.None);
        await deviceRepo.SaveAsync(staleDevice, CancellationToken.None);

        pushSender.NextInvalidTokens = new[] { "stale-token" };

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert — exactly one removal, against the stale token
        await Assert.That(deviceRepo.DeletedTokens).HasCount().EqualTo(1);
        await Assert.That(deviceRepo.DeletedTokens[0]).IsEqualTo("stale-token");
        await Assert.That(deviceRepo.GetByToken("good-token")).IsNotNull();
        await Assert.That(deviceRepo.GetByToken("stale-token")).IsNull();
    }

    [Test]
    public async Task Should_NotPruneAnyTokens_When_SenderReportsNoRejections()
    {
        // Arrange — happy path: device delivered successfully, InvalidTokens empty.
        // Idempotency guard — no spurious removal commands.
        var (handler, _, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();
        await SeedPaidUserWithDevice(userProfileRepo, deviceRepo);

        // Default NextInvalidTokens is empty.

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert — push sent, no removal calls
        await Assert.That(pushSender.Sent).HasCount().EqualTo(1);
        await Assert.That(deviceRepo.DeletedTokens).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_PruneEachInvalidToken_When_MultipleTokensRejected()
    {
        // Arrange — three devices; two reported invalid by APNs. Verifies the
        // handler iterates all entries in PushSendResult.InvalidTokens.
        var (handler, _, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        await deviceRepo.SaveAsync(
            DeviceRegistration.Create("user-1", "token-a", DevicePlatform.Ios, March2026),
            CancellationToken.None);
        await deviceRepo.SaveAsync(
            DeviceRegistration.Create("user-1", "token-b", DevicePlatform.Ios, March2026),
            CancellationToken.None);
        await deviceRepo.SaveAsync(
            DeviceRegistration.Create("user-1", "token-c", DevicePlatform.Ios, March2026),
            CancellationToken.None);

        pushSender.NextInvalidTokens = new[] { "token-a", "token-c" };

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert — both rejected tokens removed exactly once, good token retained
        await Assert.That(deviceRepo.DeletedTokens).HasCount().EqualTo(2);
        await Assert.That(deviceRepo.DeletedTokens).Contains("token-a");
        await Assert.That(deviceRepo.DeletedTokens).Contains("token-c");
        await Assert.That(deviceRepo.GetByToken("token-b")).IsNotNull();
    }

    [Test]
    public async Task Should_PassUnreadCountIncludingNewNotification_When_PushSent()
    {
        // Arrange — two pre-existing notifications strictly after the user's
        // watermark, plus the freshly-created one from this dispatch. Expected
        // badge value is 3 (2 in repo + 1 just created, not yet persisted).
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, stateRepo) = CreateHandler();
        await SeedPaidUserWithDevice(userProfileRepo, deviceRepo);

        var lastReadAt = new DateTimeOffset(2026, 3, 14, 0, 0, 0, TimeSpan.Zero);
        stateRepo.Seed(NotificationStateAggregate.Reconstitute("user-1", lastReadAt, version: 5));

        var earlyUnread = Notification.Create(
            "user-1",
            "uid-x",
            "app-x",
            "zone-1",
            "x High St",
            "X",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 14, 12, 0, 0, TimeSpan.Zero));
        var laterUnread = Notification.Create(
            "user-1",
            "uid-y",
            "app-y",
            "zone-1",
            "y High St",
            "Y",
            "Householder",
            42,
            new DateTimeOffset(2026, 3, 15, 8, 0, 0, TimeSpan.Zero));
        notificationRepo.Seed(earlyUnread);
        notificationRepo.Seed(laterUnread);

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.Sent).HasCount().EqualTo(1);
        await Assert.That(pushSender.Sent[0].TotalUnreadCount).IsEqualTo(3);
    }

    [Test]
    public async Task Should_PassUnreadCountOfOne_When_FirstTouchUserHasNoNotificationStateYet()
    {
        // Arrange — user with no notification-state document yet (first push ever).
        // Implementation defaults the watermark to DateTimeOffset.MinValue so every
        // existing row counts as unread; with zero pre-existing rows + 1 freshly
        // created notification, badge = 1.
        var (handler, _, userProfileRepo, pushSender, deviceRepo, stateRepo) = CreateHandler();
        await SeedPaidUserWithDevice(userProfileRepo, deviceRepo);

        // No stateRepo.Seed() — confirms the first-touch path.

        // Act
        await handler.HandleAsync(CreateCommand(), CancellationToken.None);

        // Assert
        await Assert.That(stateRepo.All).IsEmpty();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(1);
        await Assert.That(pushSender.Sent[0].TotalUnreadCount).IsEqualTo(1);
    }

    [Test]
    public async Task Should_RecordButNotPush_When_WatchZonePushEnabledFalse()
    {
        // Arrange — Pro user with a registered device, but the matched WatchZone
        // has PushEnabled=false (per-zone toggle from T1). Notification row must
        // still be persisted (in-app feed + weekly digest pick it up) but no
        // APNs push fires.
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _) = CreateHandler();
        await SeedPaidUserWithDevice(userProfileRepo, deviceRepo);

        var application = new PlanningApplicationBuilder()
            .WithUid("test-uid-001")
            .WithName("app-001")
            .WithCoordinates(51.5074, -0.1278)
            .Build();
        var zone = new WatchZoneBuilder()
            .WithUserId("user-1")
            .WithId("zone-1")
            .WithPushEnabled(false)
            .Build();

        // Act
        await handler.HandleAsync(new DispatchNotificationCommand(application, zone), CancellationToken.None);

        // Assert
        await Assert.That(notificationRepo.All).HasCount().EqualTo(1);
        await Assert.That(notificationRepo.All[0].PushSent).IsFalse();
        await Assert.That(pushSender.Sent).HasCount().EqualTo(0);
    }

    private static (DispatchNotificationCommandHandler Handler,
        FakeNotificationRepository NotificationRepo,
        FakeUserProfileRepository UserProfileRepo,
        SpyPushNotificationSender PushSender,
        FakeDeviceRegistrationRepository DeviceRepo,
        FakeNotificationStateRepository NotificationStateRepo) CreateHandler(FakeTimeProvider? timeProvider = null)
    {
        var notificationRepo = new FakeNotificationRepository();
        var notificationStateRepo = new FakeNotificationStateRepository();
        var userProfileRepo = new FakeUserProfileRepository();
        var deviceRepo = new FakeDeviceRegistrationRepository();
        var pushSender = new SpyPushNotificationSender();
        var removeInvalidHandler = new RemoveInvalidDeviceTokenCommandHandler(deviceRepo);
        var tp = timeProvider ?? new FakeTimeProvider(March2026);

        var handler = new DispatchNotificationCommandHandler(
            notificationRepo,
            notificationStateRepo,
            userProfileRepo,
            deviceRepo,
            pushSender,
            removeInvalidHandler,
            tp);

        return (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, notificationStateRepo);
    }

    private static async Task SeedFreeUserWithDevice(
        FakeUserProfileRepository userProfileRepo,
        FakeDeviceRegistrationRepository deviceRepo,
        string userId = "user-1")
    {
        var profile = new UserProfileBuilder()
            .WithUserId(userId)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create(userId, "device-token-1", DevicePlatform.Ios, March2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);
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
