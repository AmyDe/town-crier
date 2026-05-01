using TownCrier.Application.Notifications;
using TownCrier.Application.Tests.DeviceRegistrations;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Domain.Notifications;
using TownCrier.Domain.UserProfiles;
using FakeTimeProvider = TownCrier.Application.Tests.DeviceRegistrations.FakeTimeProvider;

namespace TownCrier.Application.Tests.Notifications;

public sealed class GenerateWeeklyDigestsCommandHandlerTests
{
    // Monday 2026-03-16 at 08:00 UTC
    private static readonly DateTimeOffset MondayMarch2026 = new(2026, 3, 16, 8, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_SendDigestPush_When_ProUserHasNotificationsThisWeek()
    {
        // Arrange
        var timeProvider = new FakeTimeProvider(MondayMarch2026);
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _, _) =
            CreateHandler(timeProvider);

        await SeedProUserWithDevice(userProfileRepo, deviceRepo, "user-1");
        SeedNotifications(notificationRepo, "user-1", count: 3, createdAt: MondayMarch2026.AddDays(-2));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(1);
        await Assert.That(pushSender.DigestsSent[0].ApplicationCount).IsEqualTo(3);
        await Assert.That(pushSender.DigestsSent[0].Devices).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_NotSendDigest_When_UserIsFreesTier()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Free)
            .WithEmailDigestEnabled(false)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-1", DevicePlatform.Ios, MondayMarch2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);
        SeedNotifications(notificationRepo, "user-1", count: 5, createdAt: MondayMarch2026.AddDays(-1));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotSendDigest_When_ZeroNewApplications()
    {
        // Arrange
        var (handler, _, userProfileRepo, pushSender, deviceRepo, _, _) = CreateHandler();
        await SeedProUserWithDevice(userProfileRepo, deviceRepo, "user-1");

        // No notifications seeded

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_OnlySendDigest_When_DigestDayMatchesToday()
    {
        // Arrange — today is Monday, user digest day is Wednesday
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _, _) = CreateHandler();
        await SeedProUserWithDevice(userProfileRepo, deviceRepo, "user-1", digestDay: DayOfWeek.Wednesday);
        SeedNotifications(notificationRepo, "user-1", count: 3, createdAt: MondayMarch2026.AddDays(-2));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotSendDigest_When_PushDisabled()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .WithPushEnabled(false)
            .WithEmailDigestEnabled(false)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-1", DevicePlatform.Ios, MondayMarch2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);
        SeedNotifications(notificationRepo, "user-1", count: 3, createdAt: MondayMarch2026.AddDays(-2));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotSendDigest_When_NoRegisteredDevices()
    {
        // Arrange — Pro user, no device registered
        var (handler, notificationRepo, userProfileRepo, pushSender, _, _, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .WithEmailDigestEnabled(false)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);
        SeedNotifications(notificationRepo, "user-1", count: 3, createdAt: MondayMarch2026.AddDays(-2));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ExcludeOldNotifications_When_OlderThanSevenDays()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _, _) = CreateHandler();
        await SeedProUserWithDevice(userProfileRepo, deviceRepo, "user-1");

        // Seed notifications older than 7 days
        SeedNotifications(notificationRepo, "user-1", count: 5, createdAt: MondayMarch2026.AddDays(-10));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert — no digest because all notifications are too old
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_SendDigestsToMultipleProUsers_When_EachHasNotifications()
    {
        // Arrange
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _, _) = CreateHandler();
        await SeedProUserWithDevice(userProfileRepo, deviceRepo, "user-1");
        await SeedProUserWithDevice(userProfileRepo, deviceRepo, "user-2");

        SeedNotifications(notificationRepo, "user-1", count: 2, createdAt: MondayMarch2026.AddDays(-1));
        SeedNotifications(notificationRepo, "user-2", count: 5, createdAt: MondayMarch2026.AddDays(-3));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_DefaultDigestDay_When_UsingDefaultPreferences()
    {
        // Arrange — user with default preferences (digest day = Monday), today is Monday
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, _, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-1", DevicePlatform.Ios, MondayMarch2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);
        SeedNotifications(notificationRepo, "user-1", count: 2, createdAt: MondayMarch2026.AddDays(-1));

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert — should send because default digest day is Monday and today is Monday
        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_SendDigestEmail_When_FreeUserHasEmailAndNotifications()
    {
        var (handler, notificationRepo, userProfileRepo, _, _, emailSender, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .WithTier(SubscriptionTier.Free)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        SeedNotificationsWithZone(
            notificationRepo,
            "user-1",
            "zone-1",
            count: 3,
            createdAt: MondayMarch2026.AddDays(-2));

        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(1);
        await Assert.That(emailSender.DigestsSent[0].Email).IsEqualTo("test@example.com");
        await Assert.That(emailSender.DigestsSent[0].Digests).HasCount().EqualTo(1);
        await Assert.That(emailSender.DigestsSent[0].Digests[0].Notifications).HasCount().EqualTo(3);
    }

    [Test]
    public async Task Should_NotSendDigestEmail_When_EmailDigestDisabled()
    {
        var (handler, notificationRepo, userProfileRepo, _, _, emailSender, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .WithEmailDigestEnabled(false)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        SeedNotificationsWithZone(
            notificationRepo,
            "user-1",
            "zone-1",
            count: 3,
            createdAt: MondayMarch2026.AddDays(-2));

        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_NotSendDigestEmail_When_NoEmailAddress()
    {
        var (handler, notificationRepo, userProfileRepo, _, _, emailSender, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithTier(SubscriptionTier.Free)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        SeedNotificationsWithZone(
            notificationRepo,
            "user-1",
            "zone-1",
            count: 3,
            createdAt: MondayMarch2026.AddDays(-2));

        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_SendBothPushAndEmail_When_ProUserHasBothEnabled()
    {
        var (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, emailSender, _) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("pro@example.com")
            .WithTier(SubscriptionTier.Pro)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create("user-1", "device-1", DevicePlatform.Ios, MondayMarch2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);

        SeedNotificationsWithZone(
            notificationRepo,
            "user-1",
            "zone-1",
            count: 2,
            createdAt: MondayMarch2026.AddDays(-1));

        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        await Assert.That(pushSender.DigestsSent).HasCount().EqualTo(1);
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_GroupNotificationsByWatchZone_When_SendingDigestEmail()
    {
        var (handler, notificationRepo, userProfileRepo, _, _, emailSender, watchZoneRepo) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var zone1 = new WatchZoneBuilder().WithId("zone-1").WithUserId("user-1").WithName("Home").Build();
        var zone2 = new WatchZoneBuilder().WithId("zone-2").WithUserId("user-1").WithName("Office").Build();
        await watchZoneRepo.SaveAsync(zone1, CancellationToken.None);
        await watchZoneRepo.SaveAsync(zone2, CancellationToken.None);

        SeedNotificationsWithZone(
            notificationRepo,
            "user-1",
            "zone-1",
            count: 2,
            createdAt: MondayMarch2026.AddDays(-1));
        SeedNotificationsWithZone(
            notificationRepo,
            "user-1",
            "zone-2",
            count: 3,
            createdAt: MondayMarch2026.AddDays(-2));

        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(1);
        var digests = emailSender.DigestsSent[0].Digests;
        await Assert.That(digests).HasCount().EqualTo(2);
        await Assert.That(digests.Any(d => d.WatchZoneName == "Home" && d.Notifications.Count == 2)).IsTrue();
        await Assert.That(digests.Any(d => d.WatchZoneName == "Office" && d.Notifications.Count == 3)).IsTrue();
    }

    [Test]
    public async Task Should_RouteBookmarkOnlyDecisionsToSavedApplications_When_NoZoneAssociation()
    {
        // Arrange — free-tier user (weekly digest is the only signal) with one
        // zone-matched new app and one bookmark-only decision.
        var (handler, notificationRepo, userProfileRepo, _, _, emailSender, watchZoneRepo) = CreateHandler();

        var profile = new UserProfileBuilder()
            .WithUserId("user-1")
            .WithEmail("test@example.com")
            .WithTier(SubscriptionTier.Free)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var zone = new WatchZoneBuilder()
            .WithId("zone-1")
            .WithUserId("user-1")
            .WithName("Home")
            .Build();
        await watchZoneRepo.SaveAsync(zone, CancellationToken.None);

        var zoneRow = Notification.Create(
            userId: "user-1",
            applicationUid: "uid-zone",
            applicationName: "APP/ZONE",
            watchZoneId: "zone-1",
            applicationAddress: "12 Acacia Ave",
            applicationDescription: "Extension",
            applicationType: "Householder",
            authorityId: 1,
            now: MondayMarch2026.AddDays(-2));
        notificationRepo.Seed(zoneRow);

        var savedRow = Notification.Create(
            userId: "user-1",
            applicationUid: "uid-saved",
            applicationName: "APP/SAVED",
            watchZoneId: null,
            applicationAddress: "9 Hill Lane",
            applicationDescription: "Change of use",
            applicationType: "Full",
            authorityId: 1,
            now: MondayMarch2026.AddDays(-1),
            decision: "Rejected",
            eventType: NotificationEventType.DecisionUpdate,
            sources: NotificationSources.Saved);
        notificationRepo.Seed(savedRow);

        // Act
        await handler.HandleAsync(new GenerateWeeklyDigestsCommand(), CancellationToken.None);

        // Assert
        await Assert.That(emailSender.DigestsSent).HasCount().EqualTo(1);
        var record = emailSender.DigestsSent[0];
        await Assert.That(record.Digests).HasCount().EqualTo(1);
        await Assert.That(record.Digests[0].WatchZoneName).IsEqualTo("Home");
        await Assert.That(record.Digests[0].Notifications).HasCount().EqualTo(1);
        await Assert.That(record.SavedApplications).HasCount().EqualTo(1);
        await Assert.That(record.SavedApplications[0].ApplicationUid).IsEqualTo("uid-saved");
    }

    private static (GenerateWeeklyDigestsCommandHandler Handler,
        FakeNotificationRepository NotificationRepo,
        FakeUserProfileRepository UserProfileRepo,
        SpyPushNotificationSender PushSender,
        FakeDeviceRegistrationRepository DeviceRepo,
        SpyEmailSender EmailSender,
        FakeWatchZoneRepository WatchZoneRepo) CreateHandler(FakeTimeProvider? timeProvider = null)
    {
        var notificationRepo = new FakeNotificationRepository();
        var userProfileRepo = new FakeUserProfileRepository();
        var deviceRepo = new FakeDeviceRegistrationRepository();
        var pushSender = new SpyPushNotificationSender();
        var emailSender = new SpyEmailSender();
        var watchZoneRepo = new FakeWatchZoneRepository();
        var tp = timeProvider ?? new FakeTimeProvider(MondayMarch2026);

        var handler = new GenerateWeeklyDigestsCommandHandler(
            userProfileRepo, notificationRepo, deviceRepo, pushSender, emailSender, watchZoneRepo, tp);

        return (handler, notificationRepo, userProfileRepo, pushSender, deviceRepo, emailSender, watchZoneRepo);
    }

    private static async Task SeedProUserWithDevice(
        FakeUserProfileRepository userProfileRepo,
        FakeDeviceRegistrationRepository deviceRepo,
        string userId,
        DayOfWeek digestDay = DayOfWeek.Monday)
    {
        var profile = new UserProfileBuilder()
            .WithUserId(userId)
            .WithEmail($"{userId}@example.com")
            .WithTier(SubscriptionTier.Pro)
            .WithDigestDay(digestDay)
            .Build();
        await userProfileRepo.SaveAsync(profile, CancellationToken.None);

        var device = DeviceRegistration.Create(userId, $"device-{userId}", DevicePlatform.Ios, MondayMarch2026);
        await deviceRepo.SaveAsync(device, CancellationToken.None);
    }

    private static void SeedNotifications(
        FakeNotificationRepository notificationRepo,
        string userId,
        int count,
        DateTimeOffset createdAt)
    {
        SeedNotificationsWithZone(notificationRepo, userId, "zone-1", count, createdAt);
    }

    private static void SeedNotificationsWithZone(
        FakeNotificationRepository notificationRepo,
        string userId,
        string watchZoneId,
        int count,
        DateTimeOffset createdAt)
    {
        for (var i = 0; i < count; i++)
        {
            var notification = Notification.Create(
                userId: userId,
                applicationUid: $"uid-{watchZoneId}-{i:D3}",
                applicationName: $"app-{watchZoneId}-{i:D3}",
                watchZoneId: watchZoneId,
                applicationAddress: $"{i} Test Street",
                applicationDescription: "Test application",
                applicationType: "Full",
                authorityId: 1,
                now: createdAt);
            notificationRepo.Seed(notification);
        }
    }
}
